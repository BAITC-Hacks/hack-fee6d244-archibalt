// Package httpapi — JSON API под /api/* (см. API_CONTRACT.md) и раздача собранного фронта.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/ai"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/rating"
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
	ProposalCounts(ctx context.Context) (map[int]int, error)
	GetTeamByContact(ctx context.Context, contact string) (model.Team, error)
	GetTeamByName(ctx context.Context, name string) (model.Team, error)
	CreateTeam(ctx context.Context, t *model.Team) error
	SetTeamContact(ctx context.Context, id int, contact string) error
	UpdateTeamProfile(ctx context.Context, id int, skills, interests, tech []string, experience, achievements string) (model.Team, error)
	ListProposalsByTeam(ctx context.Context, teamID int) ([]model.Proposal, error)
	ListTasksByOwner(ctx context.Context, contact string) ([]model.Task, error)
	SetTaskOwner(ctx context.Context, id int, contact string) error
	ListMessages(ctx context.Context, proposalID int) ([]model.Message, error)
	AddMessage(ctx context.Context, proposalID int, author, text string) (model.Message, error)
	UpdateProposal(ctx context.Context, id, teamID int, p model.Proposal) (model.Proposal, error)
	Stats(ctx context.Context) (store.Stats, error)
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

	optMu   sync.Mutex
	options map[int][]model.ResultOption // последние показанные варианты результата по задаче (в памяти, не в БД)
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
		hub: newHub(), origins: wsOrigins(os.Getenv("WS_ORIGINS")), options: map[int][]model.ResultOption{}}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/tasks", s.listTasks)
	mux.HandleFunc("POST /api/tasks", s.createTask)
	mux.HandleFunc("GET /api/tasks/{id}", s.getTask)
	mux.HandleFunc("POST /api/tasks/{id}/answers", s.answers)
	mux.HandleFunc("POST /api/tasks/{id}/next-question", s.nextQuestion)
	mux.HandleFunc("POST /api/tasks/{id}/result-options", s.resultOptions)
	mux.HandleFunc("POST /api/tasks/{id}/apply-result", s.applyResult)
	s.registerVisual(mux) // POST /visual, GET /visual.png — см. visual.go
	mux.HandleFunc("PUT /api/tasks/{id}/fields", s.updateFields)
	mux.HandleFunc("POST /api/tasks/{id}/confirm", s.confirm)
	mux.HandleFunc("POST /api/tasks/{id}/owner", s.setOwner)
	mux.HandleFunc("POST /api/tasks/{id}/proposals", s.createProposal)
	mux.HandleFunc("PUT /api/proposals/{id}", s.updateProposal)
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
	mux.HandleFunc("PUT /api/me/profile", s.updateProfile)
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

// nextLevelGain — сколько баллов до следующего уровня шкалы ТЗ §4 (40/70/90); 0 на 90+.
func nextLevelGain(score int) int {
	for _, th := range []int{40, 70, 90} {
		if score < th {
			return th - score
		}
	}
	return 0
}

// ladder — место задачи t в каталоге pub (отсортирован score DESC, published_at ASC).
// Опубликованная: rank = rankIfConfirmed = позиция. Неопубликованная: rank = 0, rankIfConfirmed —
// куда встала бы при текущем балле: после всех опубликованных с баллом >= своего (новая публикация позже).
func ladder(pub []model.Task, t model.Task) (rank, rankIfConfirmed int) {
	if t.Status == model.StatusPublished {
		for i, p := range pub {
			if p.ID == t.ID {
				return i + 1, i + 1
			}
		}
	}
	rankIfConfirmed = 1
	for _, p := range pub {
		if p.ID != t.ID && p.Score >= t.Score {
			rankIfConfirmed++
		}
	}
	return 0, rankIfConfirmed
}

// enrich заполняет вычисляемые поля «лестницы мест» у одной задачи. Ошибка каталога не ломает ответ:
// ранги остаются 0, next_level_gain считается всегда.
func (s *server) enrich(ctx context.Context, t *model.Task) {
	t.NextLevelGain = nextLevelGain(t.Score)
	pub, _, err := s.repo.ListTasks(ctx, "", "")
	if err != nil {
		slog.Warn("enrich: list tasks", "err", err)
		return
	}
	t.CatalogSize = len(pub)
	t.Rank, t.RankIfConfirmed = ladder(pub, *t)
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
	// Ранг — место во всём каталоге: при фильтре позиция в выдаче не совпадает с ним, берём полный список.
	all := tasks
	if industry != "" || level != "" {
		if all, _, err = s.repo.ListTasks(r.Context(), "", ""); err != nil {
			fail(w, err, "")
			return
		}
	}
	rankByID := make(map[int]int, len(all))
	for i, t := range all {
		rankByID[t.ID] = i + 1
	}
	counts, _ := s.repo.ProposalCounts(r.Context()) // ошибка не критична: покажем 0
	for i := range tasks {
		tasks[i].ProposalsCount = counts[tasks[i].ID]
		tasks[i].Rank = rankByID[tasks[i].ID]
		tasks[i].RankIfConfirmed = tasks[i].Rank
		tasks[i].CatalogSize = len(all)
		tasks[i].NextLevelGain = nextLevelGain(tasks[i].Score)
	}
	levels := make([]levelOption, 0, len(levelOrder))
	for _, l := range levelOrder {
		levels = append(levels, levelOption{Key: l, Label: model.LevelLabels[l]})
	}
	stats, err := s.repo.Stats(r.Context())
	if err != nil { // «пульс» — украшение: без него каталог всё равно отдаём (нули)
		slog.Warn("list tasks: stats", "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": publicTasks(tasks), "industries": industries, "levels": levels, "stats": stats})
}

// createTask: заявитель обязателен — owner_contact берётся из бизнес-сессии, поле в теле игнорируется.
func (s *server) createTask(w http.ResponseWriter, r *http.Request) {
	owner, _ := s.businessFromRequest(r) // без входа — анонимная задача: разговор сразу, вход при сборке карточки
	var req struct {
		DraftText string `json:"draft_text"`
		Industry  string `json:"industry"`
		Mode      string `json:"mode"` // "dynamic" — пошаговые вопросы (или ?mode=dynamic)
	}
	if !decode(w, r, &req) {
		return
	}
	dynamic := req.Mode == "dynamic" || r.URL.Query().Get("mode") == "dynamic"
	req.DraftText, req.Industry = strings.TrimSpace(req.DraftText), strings.TrimSpace(req.Industry)
	if req.DraftText == "" {
		writeError(w, http.StatusBadRequest, "draft_text: опишите задачу хотя бы одной фразой")
		return
	}
	if req.Industry == "" {
		writeError(w, http.StatusBadRequest, "industry: укажите отрасль")
		return
	}

	var qs []model.Question
	if dynamic {
		nr, err := s.ai.NextQuestion(r.Context(), req.DraftText, req.Industry, nil, false)
		if err != nil || nr.Question == nil {
			writeError(w, http.StatusBadGateway, "AI недоступен: не удалось получить первый вопрос")
			return
		}
		qs = normalizeQuestions([]model.Question{*nr.Question})
	} else {
		qr, err := s.ai.Questions(r.Context(), req.DraftText, req.Industry)
		if err != nil {
			writeError(w, http.StatusBadGateway, "AI недоступен: "+err.Error())
			return
		}
		qs = normalizeQuestions(qr.Questions)
	}
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
	s.enrich(r.Context(), &t)
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
	s.enrich(r.Context(), &t)
	t.HasVisual = s.hasVisual(r.Context(), t.ID)
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
	prev := t.Score // балл до пересчёта: поля уже изменены, Rating — ещё прежний
	t.Fields = t.Fields.Full()
	t.Confirmed = false
	t.Status = model.StatusEditing
	t.Rating = s.compute(t.Fields, false)
	if err := s.repo.UpdateTask(r.Context(), t); err != nil {
		fail(w, err, "задача не найдена")
		return
	}
	s.enrich(r.Context(), t)
	t.PreviousScore = &prev
	writeJSON(w, http.StatusOK, publicTask(*t))
}

func (s *server) answers(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTask(w, r)
	if !ok {
		return
	}
	if !s.ownerOrAnon(w, r, t, notEditorMsg, false) {
		return
	}
	var req struct {
		Answers map[string]string `json:"answers"`
	}
	if !decode(w, r, &req) {
		return
	}
	// пустое тело ({} или {answers:{}}) — карточка из ответов, уже сохранённых в questions (пошаговый режим)
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

// nextQuestion — пошаговый режим: записать ответ на последний вопрос (если передан; "" = пропуск),
// затем спросить AI следующий. Вопрос добавляется в task.questions; при done задача не меняется
// (кроме записанного ответа), карточку собирает POST /answers. Запрос без answer (или {"more": true}) —
// явное «спросить ещё»: done модели не мешает, следующий вопрос по пробелам вплоть до 8 всего.
func (s *server) nextQuestion(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTask(w, r)
	if !ok {
		return
	}
	if !s.ownerOrAnon(w, r, t, notEditorMsg, true) {
		return
	}
	var req struct {
		Answer *string `json:"answer"`
		More   bool    `json:"more"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "некорректное тело запроса: "+err.Error())
		return
	}
	if strings.TrimSpace(string(raw)) != "" {
		if err := json.Unmarshal(raw, &req); err != nil {
			writeError(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
			return
		}
	}
	answered := false
	if n := len(t.Questions); n > 0 && req.Answer != nil && t.Questions[n-1].Answer == "" {
		t.Questions[n-1].Answer = strings.TrimSpace(*req.Answer)
		answered = true
	}

	more := req.More || (req.Answer == nil && len(t.Questions) > 0)
	nr, err := s.ai.NextQuestion(r.Context(), t.DraftText, t.Industry, t.Questions, more)
	if err != nil {
		writeError(w, http.StatusBadGateway, "AI недоступен: "+err.Error())
		return
	}
	if nr.Question != nil {
		q := *nr.Question
		q.Answer = ""
		for _, old := range t.Questions {
			q.ID = max(q.ID, old.ID)
		}
		q.ID++
		t.Questions = append(t.Questions, q)
		nr.Question = &q
		nr.Done, nr.Reason = false, ""
	}
	if nr.Question != nil || answered {
		t.AIMode = s.ai.Mode()
		if err := s.repo.UpdateTask(r.Context(), &t); err != nil {
			fail(w, err, "задача не найдена")
			return
		}
	}
	preview := previewFields(t)
	resp := map[string]any{
		"done": nr.Done, "asked": len(t.Questions), "missing_fields": nr.MissingFields,
		"card_preview": preview, "score_preview": s.compute(preview, false).Score,
	}
	if nr.Done {
		resp["reason"] = nr.Reason
	} else {
		resp["question"] = nr.Question
	}
	writeJSON(w, http.StatusOK, resp)
}

// previewFields — дешёвая живая карточка без вызова AI: контекст = черновик (если он не бессмыслица), ответ на вопрос поля
// кладётся в это поле (несколько ответов на одно поле — через пробел). Настоящую собирает POST /answers.
func previewFields(t model.Task) model.Fields {
	f := model.Fields{}.Full()
	if !rating.IsGibberish(t.DraftText) {
		f[model.FieldContext] = t.DraftText // бессмыслица («фвфовфыов о») — не контекст, поле остаётся пробелом
	}
	seen := map[model.FieldKey]bool{}
	for _, q := range t.Questions {
		a := strings.TrimSpace(q.Answer)
		if _, ok := model.FieldLabels[q.FieldKey]; !ok || rating.IsNoise(q.FieldKey, a) {
			continue // приветствие, «не знаю», «пока ничего» — не факт о задаче, поле остаётся пробелом
		}
		if seen[q.FieldKey] {
			f[q.FieldKey] += " " + a
		} else {
			f[q.FieldKey] = a
		}
		seen[q.FieldKey] = true
	}
	return f
}

// genOptions — 2–3 варианта первого результата по черновику и ответам; запоминаются для apply-result,
// чтобы index указывал на показанный вариант (ответ модели недетерминирован).
func (s *server) genOptions(ctx context.Context, t model.Task) ([]model.ResultOption, error) {
	res, err := s.ai.ResultOptions(ctx, t.DraftText, t.Industry, t.Questions)
	if err != nil {
		return nil, err
	}
	s.optMu.Lock()
	s.options[t.ID] = res.Options
	s.optMu.Unlock()
	return res.Options, nil
}

// resultOptions — варианты первого проверяемого результата (владелец). Задачу не меняет.
func (s *server) resultOptions(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTask(w, r)
	if !ok || !s.ownerOrAnon(w, r, t, notEditorMsg, true) {
		return
	}
	opts, err := s.genOptions(r.Context(), t)
	if err != nil {
		writeError(w, http.StatusBadGateway, "AI недоступен: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"options": opts, "ai_mode": s.ai.Mode()})
}

// applyResult — перенос выбранного (и, возможно, исправленного) варианта в карточку:
// expected_result = result, success_criteria = check; дальше как PUT /fields (confirmed=false, previous_score).
func (s *server) applyResult(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTask(w, r)
	if !ok || !s.ownerOrAnon(w, r, t, notEditorMsg, false) {
		return
	}
	var req struct {
		Index *int `json:"index"`
		Edits struct {
			Result *string `json:"result"`
			Check  *string `json:"check"`
		} `json:"edits"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Index == nil {
		writeError(w, http.StatusBadRequest, "index: укажите номер выбранного варианта (с 0)")
		return
	}
	s.optMu.Lock()
	opts := s.options[t.ID]
	s.optMu.Unlock()
	if opts == nil { // после перезапуска сервера: варианты пересчитываются
		var err error
		if opts, err = s.genOptions(r.Context(), t); err != nil {
			writeError(w, http.StatusBadGateway, "AI недоступен: "+err.Error())
			return
		}
	}
	if *req.Index < 0 || *req.Index >= len(opts) {
		writeError(w, http.StatusBadRequest, "index: нет варианта с номером "+strconv.Itoa(*req.Index))
		return
	}
	o := opts[*req.Index]
	result, check := o.Result, o.Check
	if e := req.Edits.Result; e != nil && strings.TrimSpace(*e) != "" {
		result = strings.TrimSpace(*e)
	}
	if e := req.Edits.Check; e != nil && strings.TrimSpace(*e) != "" {
		check = strings.TrimSpace(*e)
	}
	t.Fields = t.Fields.Full()
	t.Fields[model.FieldExpectedResult] = result
	// Критерий, который заявитель уже написал сам, шаблон варианта не затирает — только явная правка edits.check.
	if cur := strings.TrimSpace(t.Fields[model.FieldSuccessCriteria]); cur == "" || req.Edits.Check != nil {
		t.Fields[model.FieldSuccessCriteria] = check
	}
	s.saveEdited(w, r, &t)
}

func (s *server) updateFields(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTask(w, r)
	if !ok {
		return
	}
	if !s.ownerOrAnon(w, r, t, notEditorMsg, false) {
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
	if !s.ownerOrAnon(w, r, t, notEditorMsg, false) {
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
	prev := t.Score
	t.Fields = t.Fields.Full()
	t.Confirmed = true
	t.Status = model.StatusPublished
	t.PublishedAt = &now
	t.Rating = s.compute(t.Fields, true)
	if err := s.repo.UpdateTask(r.Context(), &t); err != nil {
		fail(w, err, "задача не найдена")
		return
	}
	s.enrich(r.Context(), &t)
	t.PreviousScore = &prev
	writeJSON(w, http.StatusOK, publicTask(t))
}

const (
	badOwnerContactMsg = "owner_contact: укажите email или телефон"
	createTaskLoginMsg = "чтобы создать задачу, войдите по email или телефону"
	ownerLoginMsg      = "войдите как заявитель: по email или телефону, указанному в заявке"
	noOwnerMsg         = "у задачи нет заявителя: изменять её и принимать решения по откликам некому"
	notEditorMsg       = "изменять задачу может только её заявитель: войдите по контакту, указанному в заявке"
	notOwnerMsg        = "решение принимает заявитель задачи: войдите по контакту, указанному в заявке"
)

// requireOwner — менять задачу и решать по её откликам может только заявитель (бизнес-токен с контактом
// owner_contact). У задачи без заявителя (seed/старые) — 403 всем; без бизнес-токена — 401; чужой контакт — 403 deniedMsg.
// anonDraft — задача, начатая без входа: заявителя ещё нет, карточка не собрана.
func anonDraft(t model.Task) bool { return t.OwnerContact == "" && t.Status == model.StatusClarifying }

// ownerOrAnon — как requireOwner, но анонимный черновик: при anon разрешён без входа (диалог),
// иначе 401 «войдите» (сборка карточки, правки, публикация) — после входа задача присваивается через POST /owner.
func (s *server) ownerOrAnon(w http.ResponseWriter, r *http.Request, t model.Task, deniedMsg string, anon bool) bool {
	if anonDraft(t) {
		if anon {
			return true
		}
		if _, ok := s.businessFromRequest(r); ok {
			writeError(w, http.StatusForbidden, "сначала присвойте задачу: POST /api/tasks/{id}/owner")
			return false
		}
		writeError(w, http.StatusUnauthorized, "чтобы собрать карточку, войдите — беседа сохранится")
		return false
	}
	return s.requireOwner(w, r, t, deniedMsg)
}

func (s *server) requireOwner(w http.ResponseWriter, r *http.Request, t model.Task, deniedMsg string) bool {
	if t.OwnerContact == "" {
		writeError(w, http.StatusForbidden, noOwnerMsg)
		return false
	}
	c, ok := s.businessFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, ownerLoginMsg)
		return false
	}
	if !strings.EqualFold(c, t.OwnerContact) {
		writeError(w, http.StatusForbidden, deniedMsg)
		return false
	}
	return true
}

// setOwner меняет контакт заявителя — только сам заявитель; задаче без заявителя его не назначить.
func (s *server) setOwner(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTask(w, r)
	if !ok {
		return
	}
	var req struct {
		OwnerContact string `json:"owner_contact"`
	}
	if anonDraft(t) { // присвоить анонимную задачу вошедшему заявителю: тело пустое или {}
		c, ok := s.businessFromRequest(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "чтобы сохранить задачу за собой, войдите")
			return
		}
		if err := s.repo.SetTaskOwner(r.Context(), t.ID, c); err != nil {
			fail(w, err, "задача не найдена")
			return
		}
		t.OwnerContact = c
		s.enrich(r.Context(), &t)
		writeJSON(w, http.StatusOK, publicTask(t))
		return
	}
	if !s.requireOwner(w, r, t, "контакт заявителя может изменить только заявитель, вошедший по нему") {
		return
	}
	if !decode(w, r, &req) {
		return
	}
	c, ok := normalizeContact(req.OwnerContact)
	if !ok {
		writeError(w, http.StatusBadRequest, badOwnerContactMsg)
		return
	}
	if err := s.repo.SetTaskOwner(r.Context(), t.ID, c); err != nil {
		fail(w, err, "задача не найдена")
		return
	}
	t.OwnerContact = c
	s.enrich(r.Context(), &t)
	writeJSON(w, http.StatusOK, publicTask(t))
}

// authorizeDecision: решения по откликам (select/reject/hold/confirm-stage) принимает только заявитель задачи.
func (s *server) authorizeDecision(w http.ResponseWriter, r *http.Request, taskID int) bool {
	t, err := s.repo.GetTask(r.Context(), taskID)
	if err != nil {
		fail(w, err, "задача отклика не найдена")
		return false
	}
	return s.requireOwner(w, r, t, notOwnerMsg)
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
		Quick    bool   `json:"quick"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !ok {
		// Быстрый отклик всегда привязан к сессии: team_id из тела не может
		// подменить автора даже в демо-режиме.
		if req.Quick || RequireTeamLogin || req.TeamID <= 0 {
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
		Quick: req.Quick,
	}
	if req.Quick {
		// Контакт не попадает в снимок: после выбора общение идёт через чат.
		snapshot := team.PublicProfileSnapshot()
		p.ProfileSnapshot = &snapshot
		if p.Link != "" {
			u, err := url.Parse(p.Link)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				writeError(w, http.StatusBadRequest, "link: нужна ссылка вида https://…")
				return
			}
		}
	} else {
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

// updateProposal дополняет быстрый отклик командой. Статус, этап и снимок
// профиля не меняются: после выбора бизнеса команда продолжает тот же отклик.
func (s *server) updateProposal(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusForbidden, "редактировать можно только свой отклик")
		return
	}
	var req struct {
		Idea     *string `json:"idea"`
		Plan     *string `json:"plan"`
		Deadline *string `json:"deadline"`
		Link     *string `json:"link"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Idea != nil {
		p.Idea = strings.TrimSpace(*req.Idea)
	}
	if req.Plan != nil {
		p.Plan = strings.TrimSpace(*req.Plan)
	}
	if req.Deadline != nil {
		p.Deadline = strings.TrimSpace(*req.Deadline)
	}
	if req.Link != nil {
		p.Link = strings.TrimSpace(*req.Link)
		if p.Link != "" {
			u, err := url.Parse(p.Link)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				writeError(w, http.StatusBadRequest, "link: нужна ссылка вида https://…")
				return
			}
		}
	}
	if !p.Quick {
		var missing []string
		for _, f := range []struct {
			name string
			ok   bool
		}{{"idea", strings.TrimSpace(p.Idea) != ""}, {"plan", strings.TrimSpace(p.Plan) != ""}, {"deadline", strings.TrimSpace(p.Deadline) != ""}, {"link", strings.TrimSpace(p.Link) != ""}} {
			if !f.ok {
				missing = append(missing, f.name)
			}
		}
		if len(missing) > 0 {
			writeError(w, http.StatusBadRequest, "обязательные поля не заполнены: "+strings.Join(missing, ", "))
			return
		}
	}
	updated, err := s.repo.UpdateProposal(r.Context(), id, team.ID, p)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "отклик не найден")
			return
		}
		fail(w, err, "отклик не найден")
		return
	}
	writeJSON(w, http.StatusOK, updated)
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
		if p.Status == model.ProposalAccepted || p.Status == model.ProposalDeclined {
			writeError(w, http.StatusBadRequest, "решение по отклику уже зафиксировано командой: "+string(p.Status))
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
	if p.Status != model.ProposalAccepted && p.Status != model.ProposalSelected { // ponytail: selected допускаем, пока в UI нет кнопки «Принять»; accepted = двустороннее принятие
		writeError(w, http.StatusBadRequest, "этап можно подтвердить только у выбранной или принятой командой заявки")
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
