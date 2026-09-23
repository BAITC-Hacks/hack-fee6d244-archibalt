package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/store"
)

// DefaultDemoOTP — код входа в демо-режиме, если DEMO_OTP не задан.
const DefaultDemoOTP = "000000"

// sessions — токен → id команды.
// ponytail: сессии в памяти, перезапуск разлогинивает; при необходимости — таблица sessions
type sessions struct {
	mu sync.RWMutex
	m  map[string]int
}

func (ss *sessions) create(teamID int) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b)
	ss.mu.Lock()
	ss.m[tok] = teamID
	ss.mu.Unlock()
	return tok, nil
}

func (ss *sessions) get(tok string) (int, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	id, ok := ss.m[tok]
	return id, ok
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
	tok := bearerToken(r)
	if tok == "" {
		return model.Team{}, false
	}
	id, ok := s.sessions.get(tok)
	if !ok {
		return model.Team{}, false
	}
	t, err := s.repo.GetTeam(r.Context(), id)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("team from token", "err", err)
		}
		return model.Team{}, false
	}
	return t, true
}

func unauthorized(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "нужен вход команды")
}

const badContactMsg = "contact: укажите email или телефон"

// normalizeContact: email → lower; телефон → только цифры, с «+» если исходник начинался с «+»,
// 11 цифр с ведущей 8/7 → «+7XXXXXXXXXX». ok=false, если не похоже ни на то, ни на другое.
func normalizeContact(raw string) (string, bool) {
	c := strings.ToLower(strings.TrimSpace(raw))
	if at := strings.Index(c, "@"); at >= 0 {
		dot := strings.LastIndex(c, ".")
		if at > 0 && dot > at+1 && dot < len(c)-1 && strings.Count(c, "@") == 1 && !strings.ContainsAny(c, " \t\n") {
			return c, true
		}
		return "", false
	}
	digits := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '(', ')', '-', '+':
			return -1
		}
		return r
	}, c)
	if len(digits) < 10 || len(digits) > 15 || strings.Trim(digits, "0123456789") != "" {
		return "", false
	}
	switch {
	case len(digits) == 11 && (digits[0] == '8' || digits[0] == '7') && !strings.HasPrefix(c, "+"):
		return "+7" + digits[1:], true
	case strings.HasPrefix(c, "+"):
		return "+" + digits, true
	}
	return digits, true
}

// maskContact скрывает середину контакта для логов.
func maskContact(c string) string {
	if at := strings.Index(c, "@"); at >= 0 {
		return c[:min(2, at)] + "***" + c[at:]
	}
	if len(c) <= 4 {
		return "***"
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
	tok, err := s.sessions.create(team.ID)
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
	if teamName != "" {
		t, err := s.repo.GetTeamByName(ctx, teamName)
		switch {
		case err == nil && t.Contact == "":
			switch err := s.repo.SetTeamContact(ctx, t.ID, contact); {
			case err == nil:
				t.Contact = contact
				return t, false, nil
			case !errors.Is(err, store.ErrNotFound): // ErrNotFound: контакт успел занять другой — создаём свою
				return t, false, err
			}
		case err != nil && !errors.Is(err, store.ErrNotFound):
			return t, false, err
		}
	}
	name := teamName
	if name == "" {
		name = "Команда " + contact
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
