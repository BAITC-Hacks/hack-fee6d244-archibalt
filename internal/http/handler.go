// Package httpapi — JSON API под /api/* (см. API_CONTRACT.md) и раздача собранного фронта.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/ai"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/store"
)

// Repo — методы store.Store, нужные хендлерам (в тестах — map-реализация).
type Repo interface {
	Ping(ctx context.Context) error
	ListTasks(ctx context.Context, industry, level string) ([]model.Task, []string, error)
	GetTask(ctx context.Context, id int) (model.Task, error)
	CreateTask(ctx context.Context, t *model.Task) error
	UpdateTask(ctx context.Context, t *model.Task) error
	ListTeams(ctx context.Context) ([]model.Team, error)
	GetTeam(ctx context.Context, id int) (model.Team, error)
	CreateProposal(ctx context.Context, p *model.Proposal) error
	GetProposal(ctx context.Context, id int) (model.Proposal, error)
	UpdateProposalStatus(ctx context.Context, id int, status model.ProposalStatus) (model.Proposal, error)
	ConfirmStage(ctx context.Context, id int) (model.Proposal, error)
	GetTeamByContact(ctx context.Context, contact string) (model.Team, error)
	GetTeamByName(ctx context.Context, name string) (model.Team, error)
	CreateTeam(ctx context.Context, t *model.Team) error
	SetTeamContact(ctx context.Context, id int, contact string) error
	ListProposalsByTeam(ctx context.Context, teamID int) ([]model.Proposal, error)
	ListTasksByOwner(ctx context.Context, contact string) ([]model.Task, error)
	SetTaskOwner(ctx context.Context, id int, contact string) error
	ListMessages(ctx context.Context, proposalID int) ([]model.Message, error)
	AddMessage(ctx context.Context, proposalID int, author, text string) (model.Message, error)
}

var _ Repo = (*store.Store)(nil)

// RatingFunc — сигнатура internal/rating.Compute.
type RatingFunc func(f model.Fields, confirmed bool) model.Rating

type server struct {
	repo     Repo
	ai       ai.Client
	compute  RatingFunc
	demoOTP  string
	sessions *sessions
	hub      *hub
	origins  []string // OriginPatterns для WebSocket чата (WS_ORIGINS)
}

// NewHandler собирает роутер: /api/* + статика staticDir с SPA-fallback.
// RequireTeamLogin — требовать токен команды для отклика (REQUIRE_TEAM_LOGIN, по умолчанию true).
// false — страховка для демо без экрана входа: team_id берётся из тела запроса.
var RequireTeamLogin = true

// demoOTP — одноразовый код входа команд (пусто → DefaultDemoOTP).
func NewHandler(repo Repo, aiClient ai.Client, compute RatingFunc, staticDir, demoOTP string) http.Handler {
	if demoOTP == "" {
		demoOTP = DefaultDemoOTP
	}
	s := &server{repo: repo, ai: aiClient, compute: compute, demoOTP: demoOTP, sessions: &sessions{m: map[string]session{}},
		hub: newHub(), origins: wsOrigins(os.Getenv("WS_ORIGINS"))}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/tasks", s.listTasks)
	mux.HandleFunc("POST /api/tasks", s.createTask)
	mux.HandleFunc("GET /api/tasks/{id}", s.getTask)
	mux.HandleFunc("POST /api/tasks/{id}/answers", s.answers)
	mux.HandleFunc("PUT /api/tasks/{id}/fields", s.updateFields)
	mux.HandleFunc("POST /api/tasks/{id}/confirm", s.confirm)
	mux.HandleFunc("POST /api/tasks/{id}/owner", s.setOwner)
	mux.HandleFunc("POST /api/tasks/{id}/proposals", s.createProposal)
	mux.HandleFunc("POST /api/proposals/{id}/select", s.setProposalStatus(model.ProposalSelected))
	mux.HandleFunc("POST /api/proposals/{id}/reject", s.setProposalStatus(model.ProposalRejected))
	mux.HandleFunc("POST /api/proposals/{id}/hold", s.setProposalStatus(model.ProposalOnHold))
	mux.HandleFunc("POST /api/proposals/{id}/confirm-stage", s.confirmStage)
	mux.HandleFunc("POST /api/proposals/{id}/accept", s.teamDecision(model.ProposalAccepted))
	mux.HandleFunc("POST /api/proposals/{id}/decline", s.teamDecision(model.ProposalDeclined))
	mux.HandleFunc("GET /api/proposals/{id}/messages", s.listMessages)
	mux.HandleFunc("POST /api/proposals/{id}/messages", s.postMessage)
	mux.HandleFunc("GET /api/proposals/{id}/ws", s.chatWS)
	mux.HandleFunc("POST /api/auth/request-code", s.requestCode)
	mux.HandleFunc("POST /api/auth/verify", s.verify)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("GET /api/me", s.me)
	mux.HandleFunc("POST /api/auth/business/request-code", s.requestCode)
	mux.HandleFunc("POST /api/auth/business/verify", s.verifyBusiness)
	mux.HandleFunc("POST /api/auth/business/logout", s.logout)
	mux.HandleFunc("GET /api/business/me", s.businessMe)
	mux.HandleFunc("GET /api/teams", s.listTeams)
	mux.HandleFunc("GET /api/teams/{id}/recommended", s.recommended)
	mux.HandleFunc("GET /api/ai", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, s.ai.Info()) })
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "неизвестный API-маршрут: "+r.Method+" "+r.URL.Path)
	})
	mux.Handle("/", staticHandler(staticDir)) // "/" без метода: иначе конфликт с "/api/"

	return logRequests(mux)
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// fail переводит ошибку репозитория в HTTP-ответ.
func fail(w http.ResponseWriter, err error, notFoundMsg string) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, notFoundMsg)
		return
	}
	slog.Error("internal error", "err", err)
	writeError(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return false
	}
	return true
}

// publicTask — копия задачи для публичного ответа: owner_contact маскирован (полный — только в /api/business/me).
func publicTask(t model.Task) model.Task {
	t.OwnerContact = maskContact(t.OwnerContact)
	return t
}

func publicTasks(ts []model.Task) []model.Task {
	out := make([]model.Task, len(ts))
	for i, t := range ts {
		out[i] = publicTask(t)
	}
	return out
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "некорректный id: "+r.PathValue("id"))
		return 0, false
	}
	return id, true
}

// ---- handlers ----

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.repo.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "db": false, "ai_mode": s.ai.Mode(), "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "db": true, "ai_mode": s.ai.Mode()})
}

type levelOption struct {
	Key   model.Level `json:"key"`
	Label string      `json:"label"`
}

var levelOrder = []model.Level{model.LevelDraft, model.LevelWorking, model.LevelReady, model.LevelPriority}

func (s *server) listTasks(w http.ResponseWriter, r *http.Request) {
	industry := strings.TrimSpace(r.URL.Query().Get("industry"))
	level := strings.TrimSpace(r.URL.Query().Get("level"))
	if level != "" {
		if _, ok := model.LevelLabels[model.Level(level)]; !ok {
			writeError(w, http.StatusBadRequest, "неизвестный уровень готовности: "+level)
			return
		}
	}
	tasks, industries, err := s.repo.ListTasks(r.Context(), industry, level)
	if err != nil {
		fail(w, err, "")
		return
	}
	levels := make([]levelOption, 0, len(levelOrder))
	for _, l := range levelOrder {
		levels = append(levels, levelOption{Key: l, Label: model.LevelLabels[l]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": publicTasks(tasks), "industries": industries, "levels": levels})
}

func (s *server) createTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DraftText    string `json:"draft_text"`
		Industry     string `json:"industry"`
		OwnerContact string `json:"owner_contact"`
	}
	if !decode(w, r, &req) {
		return
	}
	var owner string
	if strings.TrimSpace(req.OwnerContact) != "" {
		c, ok := normalizeContact(req.OwnerContact)
		if !ok {
			writeError(w, http.StatusBadRequest, badOwnerContactMsg)
			return
		}
		owner = c
	}
	req.DraftText, req.Industry = strings.TrimSpace(req.DraftText), strings.TrimSpace(req.Industry)
	if req.DraftText == "" {
		writeError(w, http.StatusBadRequest, "draft_text: опишите задачу хотя бы одной фразой")
		return
	}
	if req.Industry == "" {
		writeError(w, http.StatusBadRequest, "industry: укажите отрасль")
		return
	}

	qr, err := s.ai.Questions(r.Context(), req.DraftText, req.Industry)
	if err != nil {
		writeError(w, http.StatusBadGateway, "AI недоступен: "+err.Error())
		return
	}
	qs := normalizeQuestions(qr.Questions)
	fields := model.Fields{}.Full()
	fields[model.FieldContext] = req.DraftText // черновик = контекст: даёт предварительный балл «сейчас N/100» до ответов
	t := model.Task{
		Industry:  req.Industry,
		Status:    model.StatusClarifying,
		DraftText: req.DraftText,
		Fields:    fields,
		Rating:    s.compute(fields, false),
		Questions: qs,
		AIMode:    s.ai.Mode(),

		OwnerContact: owner,
	}
	if err := s.repo.CreateTask(r.Context(), &t); err != nil {
		fail(w, err, "")
		return
	}
	writeJSON(w, http.StatusCreated, publicTask(t))
}

// normalizeQuestions проставляет уникальные id (1..n), если AI их не дал, и чистит ответы.
func normalizeQuestions(in []model.Question) []model.Question {
	out := make([]model.Question, 0, len(in))
	seen := map[int]bool{}
	for i, q := range in {
		if q.ID <= 0 || seen[q.ID] {
			q.ID = i + 1
			for seen[q.ID] {
				q.ID++
			}
		}
		seen[q.ID] = true
		q.Answer = ""
		out = append(out, q)
	}
	return out
}

func (s *server) getTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	t, err := s.repo.GetTask(r.Context(), id)
	if err != nil {
		fail(w, err, "задача не найдена")
		return
	}
	writeJSON(w, http.StatusOK, publicTask(t))
}

// loadTask — общая часть мутаций задачи: id из пути + чтение.
func (s *server) loadTask(w http.ResponseWriter, r *http.Request) (model.Task, bool) {
	id, ok := pathID(w, r)
	if !ok {
		return model.Task{}, false
	}
	t, err := s.repo.GetTask(r.Context(), id)
	if err != nil {
		fail(w, err, "задача не найдена")
		return t, false
	}
	t.Proposals = nil // мутирующие эндпоинты возвращают Task без откликов
	return t, true
}

// saveEdited: любое изменение снимает подтверждение; опубликованная задача уходит из каталога
// до повторного «Подтвердить» (баллы начисляются только подтверждённым полям, ТЗ §4).
func (s *server) saveEdited(w http.ResponseWriter, r *http.Request, t *model.Task) {
	t.Fields = t.Fields.Full()
	t.Confirmed = false
	t.Status = model.StatusEditing
	t.Rating = s.compute(t.Fields, false)
	if err := s.repo.UpdateTask(r.Context(), t); err != nil {
		fail(w, err, "задача не найдена")
		return
	}
	writeJSON(w, http.StatusOK, publicTask(*t))
}

func (s *server) answers(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTask(w, r)
	if !ok {
		return
	}
	var req struct {
		Answers map[string]string `json:"answers"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Answers == nil {
		writeError(w, http.StatusBadRequest, "answers: ожидается объект {\"<id вопроса>\": \"ответ\"}")
		return
	}
	known := map[string]bool{}
	for i := range t.Questions {
		key := strconv.Itoa(t.Questions[i].ID)
		known[key] = true
		if a, ok := req.Answers[key]; ok {
			t.Questions[i].Answer = strings.TrimSpace(a)
		}
	}
	for key := range req.Answers {
		if !known[key] {
			writeError(w, http.StatusBadRequest, "answers: нет вопроса с id "+key)
			return
		}
	}

	card, err := s.ai.Card(r.Context(), t.DraftText, t.Industry, t.Questions)
	if err != nil {
		writeError(w, http.StatusBadGateway, "AI недоступен: "+err.Error())
		return
	}
	t.Fields = card.Fields
	if t.Fields == nil {
		t.Fields = model.Fields{}
	}
	t.AIMode = s.ai.Mode()
	s.saveEdited(w, r, &t)
}

func (s *server) updateFields(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTask(w, r)
	if !ok {
		return
	}
	var req struct {
		Fields map[string]*string `json:"fields"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Fields == nil {
		writeError(w, http.StatusBadRequest, "fields: ожидается объект {\"<ключ поля>\": \"текст\"}")
		return
	}
	for k := range req.Fields {
		if _, ok := model.FieldLabels[model.FieldKey(k)]; !ok {
			writeError(w, http.StatusBadRequest, "fields: неизвестное поле "+k)
			return
		}
	}
	for k, v := range req.Fields {
		val := ""
		if v != nil {
			val = strings.TrimSpace(*v)
		}
		t.Fields[model.FieldKey(k)] = val
	}
	s.saveEdited(w, r, &t)
}

func (s *server) confirm(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTask(w, r)
	if !ok {
		return
	}
	empty := true
	for _, v := range t.Fields {
		if strings.TrimSpace(v) != "" {
			empty = false
			break
		}
	}
	if empty {
		writeError(w, http.StatusBadRequest, "карточка пуста: сначала ответьте на вопросы или заполните поля")
		return
	}
	now := time.Now()
	t.Fields = t.Fields.Full()
	t.Confirmed = true
	t.Status = model.StatusPublished
	t.PublishedAt = &now
	t.Rating = s.compute(t.Fields, true)
	if err := s.repo.UpdateTask(r.Context(), &t); err != nil {
		fail(w, err, "задача не найдена")
		return
	}
	writeJSON(w, http.StatusOK, publicTask(t))
}

const (
	badOwnerContactMsg = "owner_contact: укажите email или телефон"
	notOwnerMsg        = "решение принимает заявитель задачи: войдите по контакту, указанному в заявке"
)

// isTaskOwner — запрос с бизнес-токеном, чей контакт совпадает с owner_contact задачи.
func (s *server) isTaskOwner(r *http.Request, t model.Task) bool {
	c, ok := s.businessFromRequest(r)
	return ok && t.OwnerContact != "" && strings.EqualFold(c, t.OwnerContact)
}

// setOwner задаёт контакт заявителя: если его ещё нет — любой, иначе только бизнес с текущим контактом.
func (s *server) setOwner(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTask(w, r)
	if !ok {
		return
	}
	var req struct {
		OwnerContact string `json:"owner_contact"`
	}
	if !decode(w, r, &req) {
		return
	}
	c, ok := normalizeContact(req.OwnerContact)
	if !ok {
		writeError(w, http.StatusBadRequest, badOwnerContactMsg)
		return
	}
	// ponytail: проверка и запись не атомарны — два анонимных запроса на задачу без контакта: побеждает последний
	if t.OwnerContact != "" && !s.isTaskOwner(r, t) {
		writeError(w, http.StatusForbidden, "контакт заявителя уже задан: изменить его может только заявитель, вошедший по нему")
		return
	}
	if err := s.repo.SetTaskOwner(r.Context(), t.ID, c); err != nil {
		fail(w, err, "задача не найдена")
		return
	}
	t.OwnerContact = c
	writeJSON(w, http.StatusOK, publicTask(t))
}

// authorizeDecision: решения по откликам (select/reject/confirm-stage) у задачи с owner_contact
// принимает только её заявитель; задачи без контакта (seed) открыты, как раньше.
func (s *server) authorizeDecision(w http.ResponseWriter, r *http.Request, taskID int) bool {
	t, err := s.repo.GetTask(r.Context(), taskID)
	if err != nil {
		fail(w, err, "задача отклика не найдена")
		return false
	}
	if t.OwnerContact == "" || s.isTaskOwner(r, t) {
		return true
	}
	writeError(w, http.StatusForbidden, notOwnerMsg)
	return false
}

// createProposal: команда берётся из токена; team_id в теле игнорируется.
func (s *server) createProposal(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	team, ok := s.teamFromRequest(r)
	var req struct {
		TeamID   int    `json:"team_id"` // учитывается только при RequireTeamLogin=false (страховка для демо)
		Idea     string `json:"idea"`
		Plan     string `json:"plan"`
		Deadline string `json:"deadline"`
		Link     string `json:"link"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !ok {
		if RequireTeamLogin || req.TeamID <= 0 {
			unauthorized(w)
			return
		}
		t, err := s.repo.GetTeam(r.Context(), req.TeamID) // ponytail: REQUIRE_TEAM_LOGIN=false — отклик без входа по team_id из тела
		if err != nil {
			fail(w, err, "team_id: команда не найдена")
			return
		}
		team = t
	}
	p := model.Proposal{
		TaskID: id, Team: model.TeamRef{ID: team.ID, Name: team.Name},
		Idea: strings.TrimSpace(req.Idea), Plan: strings.TrimSpace(req.Plan),
		Deadline: strings.TrimSpace(req.Deadline), Link: strings.TrimSpace(req.Link),
	}
	var missing []string
	for _, f := range []struct {
		name string
		ok   bool
	}{{"idea", p.Idea != ""}, {"plan", p.Plan != ""}, {"deadline", p.Deadline != ""}, {"link", p.Link != ""}} {
		if !f.ok {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		writeError(w, http.StatusBadRequest, "обязательные поля не заполнены: "+strings.Join(missing, ", "))
		return
	}
	if u, err := url.Parse(p.Link); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeError(w, http.StatusBadRequest, "link: нужна ссылка вида https://…")
		return
	}

	t, err := s.repo.GetTask(r.Context(), id)
	if err != nil {
		fail(w, err, "задача не найдена")
		return
	}
	if t.Status != model.StatusPublished {
		writeError(w, http.StatusBadRequest, "задача ещё не опубликована — откликнуться можно только на задачу из каталога")
		return
	}
	if err := s.repo.CreateProposal(r.Context(), &p); err != nil {
		fail(w, err, "задача или команда не найдена")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *server) setProposalStatus(status model.ProposalStatus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		p, err := s.repo.GetProposal(r.Context(), id)
		if err != nil {
			fail(w, err, "отклик не найден")
			return
		}
		if !s.authorizeDecision(w, r, p.TaskID) {
			return
		}
		p, err = s.repo.UpdateProposalStatus(r.Context(), id, status)
		if err != nil {
			fail(w, err, "отклик не найден")
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

// teamDecision — ответ команды на выбор бизнеса: accept / decline, только команда отклика и только из selected.
func (s *server) teamDecision(status model.ProposalStatus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		team, ok := s.teamFromRequest(r)
		if !ok {
			unauthorized(w)
			return
		}
		p, err := s.repo.GetProposal(r.Context(), id)
		if err != nil {
			fail(w, err, "отклик не найден")
			return
		}
		if p.Team.ID != team.ID {
			writeError(w, http.StatusForbidden, "принять или отклонить выбор может только команда этого отклика")
			return
		}
		if p.Status != model.ProposalSelected {
			writeError(w, http.StatusBadRequest, "ответить можно только на отклик, который бизнес выбрал (status selected)")
			return
		}
		// ponytail: проверка статуса и запись не атомарны; при гонке с решением бизнеса побеждает последний
		if p, err = s.repo.UpdateProposalStatus(r.Context(), id, status); err != nil {
			fail(w, err, "отклик не найден")
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func (s *server) confirmStage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	p, err := s.repo.GetProposal(r.Context(), id)
	if err != nil {
		fail(w, err, "отклик не найден")
		return
	}
	if !s.authorizeDecision(w, r, p.TaskID) {
		return
	}
	if p.Status != model.ProposalAccepted {
		writeError(w, http.StatusBadRequest, "этап можно подтвердить только после того, как команда приняла проект")
		return
	}
	if p, err = s.repo.ConfirmStage(r.Context(), id); err != nil {
		fail(w, err, "отклик не найден")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *server) listTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := s.repo.ListTeams(r.Context())
	if err != nil {
		fail(w, err, "")
		return
	}
	for i := range teams {
		teams[i].Contact = "" // контакт — только владельцу в /api/me (ТЗ §5: персональные признаки не раскрываются)
	}
	writeJSON(w, http.StatusOK, teams)
}

func (s *server) recommended(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	team, err := s.repo.GetTeam(r.Context(), id)
	if err != nil {
		fail(w, err, "команда не найдена")
		return
	}
	tasks, _, err := s.repo.ListTasks(r.Context(), "", "")
	if err != nil {
		fail(w, err, "")
		return
	}
	writeJSON(w, http.StatusOK, publicTasks(recommend(team, tasks, 5)))
}

// ---- логирование ----

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status == 0 {
			sw.status = http.StatusOK
		}
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "status", sw.status, "ms", time.Since(start).Milliseconds())
	})
}
