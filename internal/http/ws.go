package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// Чат по отклику: REST (GET/POST /api/proposals/{id}/messages) и WebSocket (/api/proposals/{id}/ws).
// Оба пути пишут через Repo.AddMessage и рассылают новое сообщение подписчикам отклика через hub.

const (
	maxMessageRunes = 2000
	wsPingEvery     = 30 * time.Second
	wsWriteTimeout  = 10 * time.Second
	wsSendBuffer    = 32 // кадров в очереди клиента; переполнение → клиент отключается
)

// chatDenied — отказ в доступе к чату (401/403) до любых записей.
type chatDenied struct {
	status int
	msg    string
}

func (e chatDenied) Error() string { return e.msg }

func writeChatErr(w http.ResponseWriter, err error) {
	var d chatDenied
	if errors.As(err, &d) {
		writeError(w, d.status, d.msg)
		return
	}
	fail(w, err, "отклик не найден")
}

// chatAuthor — кто пишет в чат отклика p по токену tok:
// команда отклика → "team"; заявитель задачи (бизнес-токен с её owner_contact) → "business".
// Без токена → 401; чужая команда, чужой бизнес, бизнес у задачи без заявителя → 403.
func (s *server) chatAuthor(ctx context.Context, tok string, p model.Proposal) (string, error) {
	if team, ok := s.teamFromToken(ctx, tok); ok {
		if team.ID == p.Team.ID {
			return model.AuthorTeam, nil
		}
		return "", chatDenied{http.StatusForbidden, "чат доступен только команде этого отклика и заявителю задачи"}
	}
	t, err := s.repo.GetTask(ctx, p.TaskID)
	if err != nil {
		return "", err
	}
	if c, ok := s.businessFromToken(tok); ok {
		if t.OwnerContact != "" && strings.EqualFold(c, t.OwnerContact) {
			return model.AuthorBusiness, nil
		}
		return "", chatDenied{http.StatusForbidden, notOwnerMsg}
	}
	return "", chatDenied{http.StatusUnauthorized, "войдите как команда отклика или как заявитель задачи"}
}

// loadChat: id отклика из пути, отклик и автор по токену.
func (s *server) loadChat(w http.ResponseWriter, r *http.Request, tok string) (model.Proposal, string, bool) {
	id, ok := pathID(w, r)
	if !ok {
		return model.Proposal{}, "", false
	}
	p, err := s.repo.GetProposal(r.Context(), id)
	if err != nil {
		fail(w, err, "отклик не найден")
		return p, "", false
	}
	author, err := s.chatAuthor(r.Context(), tok, p)
	if err != nil {
		writeChatErr(w, err)
		return p, "", false
	}
	return p, author, true
}

// messageText: trim, непустой, не длиннее maxMessageRunes символов.
func messageText(raw string) (string, error) {
	text := strings.TrimSpace(raw)
	switch {
	case text == "":
		return "", errors.New("text: пустое сообщение")
	case utf8.RuneCountInString(text) > maxMessageRunes:
		return "", errors.New("text: не длиннее 2000 символов")
	}
	return text, nil
}

func (s *server) listMessages(w http.ResponseWriter, r *http.Request) {
	p, _, ok := s.loadChat(w, r, bearerToken(r))
	if !ok {
		return
	}
	msgs, err := s.repo.ListMessages(r.Context(), p.ID)
	if err != nil {
		fail(w, err, "отклик не найден")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (s *server) postMessage(w http.ResponseWriter, r *http.Request) {
	p, author, ok := s.loadChat(w, r, bearerToken(r))
	if !ok {
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if !decode(w, r, &req) {
		return
	}
	text, err := messageText(req.Text)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	m, err := s.repo.AddMessage(r.Context(), p.ID, author, text)
	if err != nil {
		fail(w, err, "отклик не найден")
		return
	}
	s.hub.broadcast(p.ID, wsFrame{Type: "message", Message: &m})
	writeJSON(w, http.StatusCreated, m)
}

// ---- WebSocket ----

// wsFrame — кадр протокола чата (см. API_CONTRACT.md, «WebSocket чата»).
type wsFrame struct {
	Type     string          `json:"type"` // history | message | ping | pong | error
	Messages []model.Message `json:"messages,omitempty"`
	Message  *model.Message  `json:"message,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type wsClient struct {
	send chan []byte
	kick context.CancelFunc
}

// push ставит кадр в очередь клиента; медленного клиента отключает, чтобы не блокировать рассылку.
func (c *wsClient) push(b []byte) bool {
	select {
	case c.send <- b:
		return true
	default:
		c.kick()
		return false
	}
}

// hub — подписчики чата: id отклика → соединения.
type hub struct {
	mu   sync.Mutex
	subs map[int]map[*wsClient]struct{}
}

func newHub() *hub { return &hub{subs: map[int]map[*wsClient]struct{}{}} }

func (h *hub) subscribe(pid int, c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[pid] == nil {
		h.subs[pid] = map[*wsClient]struct{}{}
	}
	h.subs[pid][c] = struct{}{}
}

func (h *hub) unsubscribe(pid int, c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs[pid], c)
	if len(h.subs[pid]) == 0 {
		delete(h.subs, pid)
	}
}

func (h *hub) broadcast(pid int, f wsFrame) {
	b, err := json.Marshal(f)
	if err != nil {
		slog.Error("ws marshal", "err", err)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.subs[pid] {
		if !c.push(b) {
			delete(h.subs[pid], c)
		}
	}
}

// wsOrigins — OriginPatterns из WS_ORIGINS (через запятую); по умолчанию локальная разработка.
// Тот же хост, что у сервера, разрешён всегда.
func wsOrigins(env string) []string {
	var out []string
	for _, p := range strings.Split(env, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		out = []string{"localhost:*", "127.0.0.1:*"}
	}
	return out
}

// chatWS — GET /api/proposals/{id}/ws?token=… Права проверяются до апгрейда (401/403/404 обычным JSON).
func (s *server) chatWS(w http.ResponseWriter, r *http.Request) {
	tok := r.URL.Query().Get("token")
	if tok == "" {
		tok = bearerToken(r)
	}
	p, author, ok := s.loadChat(w, r, tok)
	if !ok {
		return
	}
	history, err := s.repo.ListMessages(r.Context(), p.ID)
	if err != nil {
		fail(w, err, "отклик не найден")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.origins})
	if err != nil {
		slog.Warn("ws accept", "err", err) // Accept уже ответил клиенту (403 по Origin и т.п.)
		return
	}
	defer conn.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	c := &wsClient{send: make(chan []byte, wsSendBuffer), kick: cancel}
	// Подписка до отправки истории: сообщение, пришедшее между ними, может оказаться и в history, и в message —
	// клиент дедуплицирует по id. Наоборот (сначала история) — сообщение можно потерять.
	s.hub.subscribe(p.ID, c)
	defer s.hub.unsubscribe(p.ID, c)

	write := func(b []byte) error {
		wctx, wcancel := context.WithTimeout(ctx, wsWriteTimeout)
		defer wcancel()
		return conn.Write(wctx, websocket.MessageText, b)
	}
	hb, _ := json.Marshal(wsFrame{Type: "history", Messages: history})
	if err := write(hb); err != nil {
		return
	}
	pingFrame, _ := json.Marshal(wsFrame{Type: "ping"})
	go func() { // единственный писатель после истории: очередь клиента + ping каждые 30 с
		t := time.NewTicker(wsPingEvery)
		defer t.Stop()
		for {
			var b []byte
			select {
			case <-ctx.Done():
				return
			case b = <-c.send:
			case <-t.C:
				b = pingFrame
			}
			if err := write(b); err != nil {
				cancel()
				return
			}
		}
	}()

	reply := func(f wsFrame) {
		b, _ := json.Marshal(f)
		c.push(b)
	}
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() == nil && websocket.CloseStatus(err) == -1 {
				slog.Info("ws read", "proposal", p.ID, "err", err)
			}
			return
		}
		var in struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(data, &in); err != nil {
			reply(wsFrame{Type: "error", Error: "некорректный JSON: " + err.Error()})
			continue
		}
		switch in.Type {
		case "message":
			text, err := messageText(in.Text)
			if err != nil {
				reply(wsFrame{Type: "error", Error: err.Error()})
				continue
			}
			m, err := s.repo.AddMessage(ctx, p.ID, author, text)
			if err != nil {
				slog.Error("ws add message", "proposal", p.ID, "err", err)
				reply(wsFrame{Type: "error", Error: "не удалось сохранить сообщение"})
				continue
			}
			s.hub.broadcast(p.ID, wsFrame{Type: "message", Message: &m})
		case "ping":
			reply(wsFrame{Type: "pong"})
		case "pong":
		default:
			reply(wsFrame{Type: "error", Error: "неизвестный type: " + in.Type})
		}
	}
}
