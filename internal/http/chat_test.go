package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// chatFixture: опубликованная задача с owner_contact, две команды, владелец и чужой бизнес.
type chatFixture struct {
	c                          testClient
	h                          http.Handler
	repo                       *memRepo
	task                       model.Task
	teamA, teamB, owner, other string
}

func newChatFixture(t *testing.T) chatFixture {
	t.Helper()
	repo := newMemRepo()
	h := NewHandler(repo, fakeAI{}, fakeRating, t.TempDir(), "")
	f := chatFixture{c: testClient{t, h}, h: h, repo: repo}
	f.task = model.Task{Industry: "IT", Status: model.StatusPublished, Fields: model.Fields{}.Full(), OwnerContact: "owner@mail.kz"}
	if err := repo.CreateTask(t.Context(), &f.task); err != nil {
		t.Fatal(err)
	}
	var lg struct{ Token string }
	for _, x := range []struct {
		dst  *string
		path string
		c    string
	}{
		{&f.teamA, "/api/auth/verify", "a@team.kz"}, {&f.teamB, "/api/auth/verify", "b@team.kz"},
		{&f.owner, "/api/auth/business/verify", "owner@mail.kz"}, {&f.other, "/api/auth/business/verify", "other@mail.kz"},
	} {
		f.c.do("POST", x.path, map[string]string{"contact": x.c, "code": "000000"}, 200, &lg)
		*x.dst = lg.Token
	}
	return f
}

func (f chatFixture) propose(tok string, taskID int) model.Proposal {
	var p model.Proposal
	f.c.doAuth(tok, "POST", fmt.Sprintf("/api/tasks/%d/proposals", taskID),
		map[string]any{"idea": "Бот", "plan": "2 спринта", "deadline": "3 недели", "link": "https://example.com"}, 201, &p)
	return p
}

func TestTeamDecisions(t *testing.T) {
	f := newChatFixture(t)
	c := f.c
	pa := f.propose(f.teamA, f.task.ID)
	a := fmt.Sprintf("/api/proposals/%d", pa.ID)

	c.doAuth(f.teamA, "POST", a+"/accept", nil, 400, nil) // ещё new
	c.doAuth(f.owner, "POST", a+"/select", nil, 200, nil)
	c.do("POST", a+"/accept", nil, 401, nil)
	c.doAuth(f.owner, "POST", a+"/accept", nil, 401, nil) // бизнес-токен — не команда
	c.doAuth(f.teamB, "POST", a+"/accept", nil, 403, nil)
	c.doAuth(f.owner, "POST", a+"/confirm-stage", nil, 200, nil) // выбрана — этап можно подтвердить и до принятия (UI без кнопки «Принять»)
	var p model.Proposal
	c.doAuth(f.teamA, "POST", a+"/accept", nil, 200, &p)
	if p.Status != model.ProposalAccepted || p.AcceptedAt == nil {
		t.Fatalf("accept: %+v", p)
	}
	c.doAuth(f.teamA, "POST", a+"/decline", nil, 400, nil) // уже accepted
	c.doAuth(f.owner, "POST", a+"/confirm-stage", nil, 200, &p)
	if !p.StageConfirmed {
		t.Fatalf("confirm-stage после accept: %+v", p)
	}

	pb := f.propose(f.teamB, f.task.ID)
	b := fmt.Sprintf("/api/proposals/%d", pb.ID)
	c.doAuth(f.owner, "POST", b+"/select", nil, 200, nil)
	c.doAuth(f.teamA, "POST", b+"/decline", nil, 403, nil)
	c.doAuth(f.teamB, "POST", b+"/decline", nil, 200, &p)
	if p.Status != model.ProposalDeclined || p.AcceptedAt != nil {
		t.Fatalf("decline: %+v", p)
	}
	c.doAuth(f.teamB, "POST", b+"/accept", nil, 400, nil)
	c.doAuth(f.owner, "POST", b+"/confirm-stage", nil, 400, nil)

	// hold — бизнес по правилам select
	c.do("POST", b+"/hold", nil, 403, nil)
	c.doAuth(f.other, "POST", b+"/hold", nil, 403, nil)
	c.doAuth(f.owner, "POST", b+"/hold", nil, 200, &p)
	if p.Status != model.ProposalOnHold {
		t.Fatalf("hold: %+v", p)
	}
	c.do("POST", "/api/proposals/99/accept", nil, 401, nil)
	c.doAuth(f.teamA, "POST", "/api/proposals/99/accept", nil, 404, nil)
}

func TestChatREST(t *testing.T) {
	f := newChatFixture(t)
	c := f.c
	pa := f.propose(f.teamA, f.task.ID)
	mpath := fmt.Sprintf("/api/proposals/%d/messages", pa.ID)

	c.do("POST", mpath, map[string]string{"text": "привет"}, 401, nil)
	c.do("GET", mpath, nil, 401, nil)
	c.doAuth(f.teamB, "POST", mpath, map[string]string{"text": "чужая команда"}, 403, nil)
	c.doAuth(f.other, "GET", mpath, nil, 403, nil)
	c.doAuth(f.teamA, "POST", mpath, map[string]string{"text": "   "}, 400, nil)
	c.doAuth(f.teamA, "POST", mpath, map[string]string{"text": strings.Repeat("я", 2001)}, 400, nil)
	c.doAuth(f.teamA, "POST", mpath, map[string]string{"text": strings.Repeat("я", 2000)}, 201, nil)

	var m model.Message
	c.doAuth(f.teamA, "POST", mpath, map[string]string{"text": "  Когда созвон?  "}, 201, &m)
	if m.Author != model.AuthorTeam || m.Text != "Когда созвон?" || m.ID == 0 {
		t.Fatalf("сообщение команды: %+v", m)
	}
	c.doAuth(f.owner, "POST", mpath, map[string]string{"text": "Завтра в 10"}, 201, &m)
	if m.Author != model.AuthorBusiness {
		t.Fatalf("сообщение бизнеса: %+v", m)
	}
	var list struct{ Messages []model.Message }
	c.doAuth(f.teamA, "GET", mpath, nil, 200, &list)
	if len(list.Messages) != 3 || list.Messages[1].Author != model.AuthorTeam || list.Messages[2].Text != "Завтра в 10" ||
		list.Messages[1].CreatedAt.After(list.Messages[2].CreatedAt) {
		t.Fatalf("список сообщений: %+v", list.Messages)
	}

	// messages_count в откликах
	var me struct{ Proposals []MyProposal }
	c.doAuth(f.teamA, "GET", "/api/me", nil, 200, &me)
	if len(me.Proposals) != 1 || me.Proposals[0].MessagesCount != 3 {
		t.Fatalf("messages_count в /api/me: %+v", me.Proposals)
	}
	var bme struct{ Tasks []model.Task }
	c.doAuth(f.owner, "GET", "/api/business/me", nil, 200, &bme)
	if len(bme.Tasks) != 1 || bme.Tasks[0].Proposals[0].MessagesCount != 3 {
		t.Fatalf("messages_count в /api/business/me: %+v", bme.Tasks)
	}

	// задача без контакта: без токена пишет «бизнес» (демо), любой бизнес тоже
	seed := model.Task{Industry: "IT", Status: model.StatusPublished, Fields: model.Fields{}.Full()}
	if err := f.repo.CreateTask(t.Context(), &seed); err != nil {
		t.Fatal(err)
	}
	sp := f.propose(f.teamA, seed.ID)
	spath := fmt.Sprintf("/api/proposals/%d/messages", sp.ID)
	c.do("POST", spath, map[string]string{"text": "демо"}, 201, &m)
	if m.Author != model.AuthorBusiness {
		t.Fatalf("демо-автор: %+v", m)
	}
	c.doAuth(f.other, "GET", spath, nil, 200, nil)
	c.doAuth(f.teamB, "GET", spath, nil, 403, nil)
	c.do("GET", "/api/proposals/99/messages", nil, 404, nil)
}

func readFrame(t *testing.T, conn *websocket.Conn) wsFrame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	var f wsFrame
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("ws frame %q: %v", data, err)
	}
	return f
}

func TestChatWS(t *testing.T) {
	f := newChatFixture(t)
	srv := httptest.NewServer(f.h)
	defer srv.Close()
	pa := f.propose(f.teamA, f.task.ID)
	mpath := fmt.Sprintf("/api/proposals/%d/messages", pa.ID)
	f.c.doAuth(f.teamA, "POST", mpath, map[string]string{"text": "до подключения"}, 201, nil)

	wsURL := func(tok string) string {
		return fmt.Sprintf("ws%s/api/proposals/%d/ws?token=%s", strings.TrimPrefix(srv.URL, "http"), pa.ID, tok)
	}
	ctx := t.Context()

	// без прав — отказ до апгрейда
	for tok, want := range map[string]int{"": 401, f.teamB: 403, f.other: 403} {
		_, resp, err := websocket.Dial(ctx, wsURL(tok), nil)
		if err == nil || resp == nil || resp.StatusCode != want {
			code := 0
			if resp != nil {
				code = resp.StatusCode
			}
			t.Fatalf("ws без прав (%q): err=%v status=%d, want %d", tok, err, code, want)
		}
	}
	// чужой Origin
	_, resp, err := websocket.Dial(ctx, wsURL(f.teamA), &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://evil.example"}}})
	if err == nil || resp == nil || resp.StatusCode != 403 {
		t.Fatalf("чужой Origin должен давать 403: %v", err)
	}

	team, _, err := websocket.Dial(ctx, wsURL(f.teamA), &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"http://localhost:5173"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer team.CloseNow()
	if h := readFrame(t, team); h.Type != "history" || len(h.Messages) != 1 || h.Messages[0].Text != "до подключения" {
		t.Fatalf("history: %+v", h)
	}

	// REST от бизнеса доставляется в WS
	f.c.doAuth(f.owner, "POST", mpath, map[string]string{"text": "из REST"}, 201, nil)
	if fr := readFrame(t, team); fr.Type != "message" || fr.Message == nil || fr.Message.Author != model.AuthorBusiness || fr.Message.Text != "из REST" {
		t.Fatalf("REST → WS: %+v", fr)
	}

	biz, _, err := websocket.Dial(ctx, wsURL(f.owner), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer biz.CloseNow()
	if h := readFrame(t, biz); h.Type != "history" || len(h.Messages) != 2 {
		t.Fatalf("history бизнеса: %+v", h)
	}

	// сообщение из WS сохраняется и доходит второму подписчику (и отправителю)
	if err := team.Write(ctx, websocket.MessageText, []byte(`{"type":"message","text":"  из WS  "}`)); err != nil {
		t.Fatal(err)
	}
	for _, conn := range []*websocket.Conn{biz, team} {
		if fr := readFrame(t, conn); fr.Type != "message" || fr.Message.Author != model.AuthorTeam || fr.Message.Text != "из WS" {
			t.Fatalf("WS → подписчики: %+v", fr)
		}
	}
	msgs, _ := f.repo.ListMessages(ctx, pa.ID)
	if len(msgs) != 3 || msgs[2].Text != "из WS" || msgs[2].Author != model.AuthorTeam {
		t.Fatalf("сообщение из WS не сохранено: %+v", msgs)
	}

	// ошибки протокола не рвут соединение
	for _, bad := range []string{`{broken`, `{"type":"message","text":"  "}`, `{"type":"what"}`} {
		if err := team.Write(ctx, websocket.MessageText, []byte(bad)); err != nil {
			t.Fatal(err)
		}
		if fr := readFrame(t, team); fr.Type != "error" || fr.Error == "" {
			t.Fatalf("%s → ожидался error: %+v", bad, fr)
		}
	}
	if err := team.Write(ctx, websocket.MessageText, []byte(`{"type":"ping"}`)); err != nil {
		t.Fatal(err)
	}
	if fr := readFrame(t, team); fr.Type != "pong" {
		t.Fatalf("ping → pong: %+v", fr)
	}

	// отписка при закрытии: рассылка после закрытия не ломает хаб
	team.Close(websocket.StatusNormalClosure, "")
	f.c.doAuth(f.owner, "POST", mpath, map[string]string{"text": "после закрытия"}, 201, nil)
	if fr := readFrame(t, biz); fr.Message == nil || fr.Message.Text != "после закрытия" {
		t.Fatalf("после закрытия одного подписчика: %+v", fr)
	}
}
