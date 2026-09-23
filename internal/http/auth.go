package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/store"
)

// DefaultDemoOTP — код входа в демо-режиме, если DEMO_OTP не задан.
const DefaultDemoOTP = "000000"

// Роли сессии: вход команды и вход бизнеса (заявителя задачи) одним и тем же одноразовым кодом.
const (
	kindTeam     = "team"
	kindBusiness = "business"
)

type session struct {
	kind    string // kindTeam | kindBusiness
	teamID  int    // для kindTeam
	contact string // для kindBusiness — нормализованный контакт заявителя
}

// sessions — токен → сессия.
// ponytail: сессии в памяти, перезапуск разлогинивает; при необходимости — таблица sessions
type sessions struct {
	mu sync.RWMutex
	m  map[string]session
}

func (ss *sessions) create(sess session) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b)
	ss.mu.Lock()
	ss.m[tok] = sess
	ss.mu.Unlock()
	return tok, nil
}

func (ss *sessions) get(tok string) (session, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	sess, ok := ss.m[tok]
	return sess, ok
}

func (ss *sessions) remove(tok string) {
	ss.mu.Lock()
	delete(ss.m, tok)
	ss.mu.Unlock()
}

func bearerToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// teamFromRequest — команда по заголовку Authorization: Bearer <token>.
func (s *server) teamFromRequest(r *http.Request) (model.Team, bool) {
	return s.teamFromToken(r.Context(), bearerToken(r))
}

// teamFromToken — команда по токену сессии kind=team.
func (s *server) teamFromToken(ctx context.Context, tok string) (model.Team, bool) {
	if tok == "" {
		return model.Team{}, false
	}
	sess, ok := s.sessions.get(tok)
	if !ok || sess.kind != kindTeam {
		return model.Team{}, false
	}
	t, err := s.repo.GetTeam(ctx, sess.teamID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("team from token", "err", err)
		}
		return model.Team{}, false
	}
	return t, true
}

// businessFromRequest — контакт заявителя по заголовку Authorization: Bearer <token> бизнес-сессии.
func (s *server) businessFromRequest(r *http.Request) (string, bool) {
	return s.businessFromToken(bearerToken(r))
}

// businessFromToken — контакт заявителя по токену сессии kind=business.
func (s *server) businessFromToken(tok string) (string, bool) {
	if tok == "" {
		return "", false
	}
	sess, ok := s.sessions.get(tok)
	if !ok || sess.kind != kindBusiness {
		return "", false
	}
	return sess.contact, true
}

func unauthorized(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "нужен вход команды")
}

const badContactMsg = "contact: укажите email или телефон"

// normalizeContact — см. store.NormalizeContact (одна нормализация для входа и seed).
func normalizeContact(raw string) (string, bool) { return store.NormalizeContact(raw) }

// maskContact скрывает середину контакта для логов и публичных ответов (owner_contact задачи):
// «o***@mail.kz», «+7701***0099».
func maskContact(c string) string {
	if c == "" {
		return ""
	}
	if at := strings.Index(c, "@"); at >= 0 {
		return c[:min(1, at)] + "***" + c[at:]
	}
	switch {
	case len(c) <= 4:
		return "***"
	case len(c) >= 11:
		return c[:5] + "***" + c[len(c)-4:]
	}
	return c[:2] + "***" + c[len(c)-4:]
}

func (s *server) requestCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Contact string `json:"contact"`
	}
	if !decode(w, r, &req) {
		return
	}
	contact, ok := normalizeContact(req.Contact)
	if !ok {
		writeError(w, http.StatusBadRequest, badContactMsg)
		return
	}
	slog.Info("otp requested", "contact", maskContact(contact))
	writeJSON(w, http.StatusOK, map[string]any{
		"sent": true, "demo": true,
		"hint": "Демо-режим: код никуда не отправляется, введите " + s.demoOTP,
	})
}

func (s *server) verify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Contact  string `json:"contact"`
		Code     string `json:"code"`
		TeamName string `json:"team_name"`
	}
	if !decode(w, r, &req) {
		return
	}
	contact, ok := normalizeContact(req.Contact)
	if !ok {
		writeError(w, http.StatusBadRequest, badContactMsg)
		return
	}
	if strings.TrimSpace(req.Code) != s.demoOTP {
		writeError(w, http.StatusUnauthorized, "неверный код")
		return
	}
	team, created, err := s.teamForContact(r, contact, strings.TrimSpace(req.TeamName))
	if err != nil {
		fail(w, err, "")
		return
	}
	tok, err := s.sessions.create(session{kind: kindTeam, teamID: team.ID})
	if err != nil {
		fail(w, err, "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": tok, "team": team, "created": created})
}

// teamForContact: команда с этим контактом → она; иначе seed-команда с именем teamName и без контакта
// получает контакт; иначе создаётся новая команда.
func (s *server) teamForContact(r *http.Request, contact, teamName string) (model.Team, bool, error) {
	ctx := r.Context()
	t, err := s.repo.GetTeamByContact(ctx, contact)
	if err == nil {
		return t, false, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return t, false, err
	}
	// Привязка к существующей команде по имени убрана: иначе любой мог войти в чужую (seed) команду по имени и коду 000000.
	// Незнакомый контакт всегда получает свою команду; team_name — только имя новой.
	name := teamName
	if name == "" {
		name = "Команда " + tokenSuffix(contact) // без телефона/email в публичном имени
	}
	nt := model.Team{Name: name, Contact: contact}
	if err := s.repo.CreateTeam(ctx, &nt); err != nil {
		// гонка двух verify с одним контактом: уникальный индекс не дал вставить — берём победителя
		if t, gerr := s.repo.GetTeamByContact(ctx, contact); gerr == nil {
			return t, false, nil
		}
		return nt, false, err
	}
	return nt, true, nil
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if tok := bearerToken(r); tok != "" {
		s.sessions.remove(tok)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type myTask struct {
	ID         int              `json:"id"`
	Title      string           `json:"title"`
	Industry   string           `json:"industry"`
	Score      int              `json:"score"`
	Level      model.Level      `json:"level"`
	LevelLabel string           `json:"level_label"`
	Status     model.TaskStatus `json:"status"`
}

// MyProposal — отклик команды вместе с краткой карточкой задачи («Мои отклики»).
type MyProposal struct {
	model.Proposal
	Task myTask `json:"task"`
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	team, ok := s.teamFromRequest(r)
	if !ok {
		unauthorized(w)
		return
	}
	props, err := s.repo.ListProposalsByTeam(r.Context(), team.ID)
	if err != nil {
		fail(w, err, "")
		return
	}
	tasks := map[int]myTask{}
	out := make([]MyProposal, 0, len(props))
	for _, p := range props {
		mt, ok := tasks[p.TaskID]
		if !ok {
			t, err := s.repo.GetTask(r.Context(), p.TaskID)
			if err != nil {
				fail(w, err, "задача отклика не найдена")
				return
			}
			mt = myTask{ID: t.ID, Title: t.Fields[model.FieldTitle], Industry: t.Industry, Score: t.Score,
				Level: t.Level, LevelLabel: t.LevelLabel, Status: t.Status}
			tasks[p.TaskID] = mt
		}
		out = append(out, MyProposal{Proposal: p, Task: mt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"team": team, "proposals": out})
}

// verifyBusiness — вход заявителя: тот же код, ничего не создаёт; задачи находятся по owner_contact.
func (s *server) verifyBusiness(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Contact string `json:"contact"`
		Code    string `json:"code"`
	}
	if !decode(w, r, &req) {
		return
	}
	contact, ok := normalizeContact(req.Contact)
	if !ok {
		writeError(w, http.StatusBadRequest, badContactMsg)
		return
	}
	if strings.TrimSpace(req.Code) != s.demoOTP {
		writeError(w, http.StatusUnauthorized, "неверный код")
		return
	}
	tok, err := s.sessions.create(session{kind: kindBusiness, contact: contact})
	if err != nil {
		fail(w, err, "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": tok, "contact": contact})
}

// businessMe — задачи заявителя с полным owner_contact и откликами («Мои задачи»).
func (s *server) businessMe(w http.ResponseWriter, r *http.Request) {
	contact, ok := s.businessFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "нужен вход бизнеса")
		return
	}
	tasks, err := s.repo.ListTasksByOwner(r.Context(), contact)
	if err != nil {
		fail(w, err, "")
		return
	}
	out := make([]ownerTask, len(tasks))
	for i, t := range tasks {
		s.enrich(r.Context(), &t) // ponytail: ListTasks на каждую задачу; задач у заявителя единицы
		out[i] = ownerTask{Task: t, Proposals: t.Proposals}
		if out[i].Proposals == nil {
			out[i].Proposals = []model.Proposal{}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"contact": contact, "tasks": out})
}

// ownerTask — задача в «Мои задачи»: proposals всегда массивом (у model.Task он omitempty), даже пустым.
type ownerTask struct {
	model.Task
	Proposals []model.Proposal `json:"proposals"`
}

// tokenSuffix — короткий стабильный код из контакта для имени команды по умолчанию, чтобы не публиковать телефон.
func tokenSuffix(contact string) string {
	sum := sha256.Sum256([]byte(contact))
	return fmt.Sprintf("%04d", (int(sum[0])<<8|int(sum[1]))%10000)
}
