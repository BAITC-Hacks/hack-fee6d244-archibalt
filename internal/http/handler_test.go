package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/ai"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/store"
)

// memRepo — map-реализация Repo для тестов хендлеров без Postgres.
type memRepo struct {
	mu        sync.Mutex
	tasks     map[int]model.Task
	teams     map[int]model.Team
	proposals map[int]model.Proposal
	nextTask  int
	nextProp  int
}

func newMemRepo() *memRepo {
	return &memRepo{
		tasks:     map[int]model.Task{},
		teams:     map[int]model.Team{1: {ID: 1, Name: "Байты", Skills: []string{"Go"}, Interests: []string{"логистика"}, Tech: []string{"Postgres"}}},
		proposals: map[int]model.Proposal{},
	}
}

func (m *memRepo) Ping(context.Context) error { return nil }

func (m *memRepo) ListTasks(_ context.Context, industry, level string) ([]model.Task, []string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []model.Task{}
	inds := map[string]bool{}
	for _, t := range m.tasks {
		if t.Status != model.StatusPublished {
			continue
		}
		inds[t.Industry] = true
		if (industry == "" || t.Industry == industry) && (level == "" || string(t.Level) == level) {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	industries := []string{}
	for k := range inds {
		industries = append(industries, k)
	}
	sort.Strings(industries)
	return out, industries, nil
}

func (m *memRepo) GetTask(_ context.Context, id int) (model.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return t, store.ErrNotFound
	}
	t.Proposals = []model.Proposal{}
	for _, p := range m.proposals {
		if p.TaskID == id {
			t.Proposals = append(t.Proposals, p)
		}
	}
	return t, nil
}

func (m *memRepo) CreateTask(_ context.Context, t *model.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextTask++
	t.ID, t.CreatedAt = m.nextTask, time.Now()
	m.tasks[t.ID] = *t
	return nil
}

func (m *memRepo) UpdateTask(_ context.Context, t *model.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[t.ID]; !ok {
		return store.ErrNotFound
	}
	cp := *t
	cp.Fields = t.Fields.Full()
	m.tasks[t.ID] = cp
	return nil
}

func (m *memRepo) ListTeams(context.Context) ([]model.Team, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []model.Team{}
	for _, t := range m.teams {
		out = append(out, t)
	}
	return out, nil
}

func (m *memRepo) GetTeam(_ context.Context, id int) (model.Team, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.teams[id]
	if !ok {
		return t, store.ErrNotFound
	}
	return t, nil
}

func (m *memRepo) CreateProposal(_ context.Context, p *model.Proposal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	team, ok := m.teams[p.Team.ID]
	if _, tok := m.tasks[p.TaskID]; !ok || !tok {
		return store.ErrNotFound
	}
	m.nextProp++
	p.ID, p.Team.Name, p.Status, p.CreatedAt = m.nextProp, team.Name, model.ProposalNew, time.Now()
	m.proposals[p.ID] = *p
	return nil
}

func (m *memRepo) GetProposal(_ context.Context, id int) (model.Proposal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.proposals[id]
	if !ok {
		return p, store.ErrNotFound
	}
	return p, nil
}

func (m *memRepo) UpdateProposalStatus(_ context.Context, id int, st model.ProposalStatus) (model.Proposal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.proposals[id]
	if !ok {
		return p, store.ErrNotFound
	}
	p.Status = st
	m.proposals[id] = p
	return p, nil
}

func (m *memRepo) ConfirmStage(_ context.Context, id int) (model.Proposal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.proposals[id]
	if !ok {
		return p, store.ErrNotFound
	}
	if !p.StageConfirmed {
		p.StageConfirmed = true
		team := m.teams[p.Team.ID]
		team.Points += store.StagePoints
		m.teams[p.Team.ID] = team
		m.proposals[id] = p
	}
	return p, nil
}

// fakeRating: 10 баллов за каждое непустое поле.
func fakeRating(f model.Fields, _ bool) model.Rating {
	score := 0
	for _, v := range f {
		if v != "" {
			score += 10
		}
	}
	return model.Rating{Score: score, Level: model.LevelDraft, LevelLabel: "черновик", Breakdown: []model.BreakdownItem{}, Missing: []model.MissingItem{}}
}

type testClient struct {
	t *testing.T
	h http.Handler
}

func (c testClient) do(method, path string, body any, wantStatus int, out any) {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if s, ok := body.(string); ok {
			buf.WriteString(s)
		} else if err := json.NewEncoder(&buf).Encode(body); err != nil {
			c.t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		c.t.Fatalf("%s %s: status %d, want %d; body: %s", method, path, rec.Code, wantStatus, rec.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			c.t.Fatalf("%s %s: decode: %v; body: %s", method, path, err, rec.Body.String())
		}
	}
}

func TestFlow(t *testing.T) {
	repo := newMemRepo()
	c := testClient{t, NewHandler(repo, fakeAI{}, fakeRating, t.TempDir())}

	var errResp struct{ Error string }
	c.do("POST", "/api/tasks", map[string]string{"draft_text": "  ", "industry": "IT"}, 400, &errResp)
	if errResp.Error == "" {
		t.Fatal("ожидалось поле error")
	}
	c.do("POST", "/api/tasks", map[string]string{"draft_text": "Нужно приложение", "industry": ""}, 400, nil)
	c.do("POST", "/api/tasks", "{broken", 400, nil)

	var task model.Task
	c.do("POST", "/api/tasks", map[string]string{"draft_text": "Нужно приложение для учёта заявок", "industry": "IT"}, 201, &task)
	if task.ID == 0 || task.Status != model.StatusClarifying || len(task.Questions) < 3 || task.AIMode != model.AIModeMock {
		t.Fatalf("неверная задача после создания: %+v", task)
	}
	if len(task.Fields) != len(model.FieldKeys) {
		t.Fatalf("fields должны содержать все 10 ключей, got %d", len(task.Fields))
	}
	path := fmt.Sprintf("/api/tasks/%d", task.ID)

	// до публикации задачи нет в каталоге и откликнуться нельзя
	var cat struct {
		Tasks      []model.Task
		Industries []string
		Levels     []struct{ Key, Label string }
	}
	c.do("GET", "/api/tasks", nil, 200, &cat)
	if len(cat.Tasks) != 0 || len(cat.Levels) != 4 {
		t.Fatalf("каталог до публикации: %+v", cat)
	}
	c.do("POST", "/api/tasks/99/answers", map[string]any{"answers": map[string]string{}}, 404, nil)

	answers := map[string]string{}
	for _, q := range task.Questions {
		answers[fmt.Sprint(q.ID)] = "ответ на " + string(q.FieldKey)
	}
	c.do("POST", path+"/answers", map[string]any{"answers": map[string]string{"999": "x"}}, 400, nil)
	c.do("POST", path+"/answers", map[string]any{"answers": answers}, 200, &task)
	if task.Status != model.StatusEditing || task.Confirmed || task.Fields[model.FieldUsers] != "ответ на users" || task.Questions[0].Answer == "" {
		t.Fatalf("после answers: %+v", task)
	}
	before := task.Score

	c.do("PUT", path+"/fields", map[string]any{"fields": map[string]string{"bogus": "x"}}, 400, nil)
	c.do("PUT", path+"/fields", map[string]any{"fields": map[string]string{"interaction_format": "раз в неделю созвон"}}, 200, &task)
	if task.Fields[model.FieldInteractionFormat] != "раз в неделю созвон" || task.Fields[model.FieldUsers] == "" || task.Score <= before {
		t.Fatalf("PUT fields должен мерджить и пересчитывать: score %d→%d, %+v", before, task.Score, task.Fields)
	}

	c.do("POST", path+"/proposals", map[string]any{"team_id": 1, "idea": "i", "plan": "p", "deadline": "d", "link": "l"}, 400, nil) // не опубликована

	c.do("POST", path+"/confirm", nil, 200, &task)
	if !task.Confirmed || task.Status != model.StatusPublished || task.PublishedAt == nil {
		t.Fatalf("после confirm: %+v", task)
	}
	c.do("GET", "/api/tasks?industry=IT", nil, 200, &cat)
	if len(cat.Tasks) != 1 || cat.Tasks[0].ID != task.ID || len(cat.Industries) != 1 {
		t.Fatalf("после confirm задача должна быть в каталоге: %+v", cat)
	}
	c.do("GET", "/api/tasks?industry=Медицина", nil, 200, &cat)
	if len(cat.Tasks) != 0 {
		t.Fatal("фильтр по отрасли не работает")
	}
	c.do("GET", "/api/tasks?level=bogus", nil, 400, nil)

	c.do("POST", path+"/proposals", map[string]any{"team_id": 1, "idea": "idea"}, 400, nil)
	c.do("POST", path+"/proposals", map[string]any{"team_id": 42, "idea": "i", "plan": "p", "deadline": "d", "link": "l"}, 400, nil)
	var prop model.Proposal
	c.do("POST", path+"/proposals", map[string]any{"team_id": 1, "idea": "Бот", "plan": "2 спринта", "deadline": "3 недели", "link": "https://example.com"}, 201, &prop)
	if prop.ID == 0 || prop.Team.Name != "Байты" || prop.Status != model.ProposalNew {
		t.Fatalf("proposal: %+v", prop)
	}
	ppath := fmt.Sprintf("/api/proposals/%d", prop.ID)
	c.do("POST", ppath+"/confirm-stage", nil, 400, nil) // ещё не выбрана
	c.do("POST", ppath+"/select", nil, 200, &prop)
	if prop.Status != model.ProposalSelected {
		t.Fatalf("select: %+v", prop)
	}
	c.do("POST", ppath+"/confirm-stage", nil, 200, &prop)
	c.do("POST", ppath+"/confirm-stage", nil, 200, &prop) // идемпотентно
	if !prop.StageConfirmed || repo.teams[1].Points != store.StagePoints {
		t.Fatalf("confirm-stage: %+v, points %d", prop, repo.teams[1].Points)
	}
	c.do("POST", "/api/proposals/77/reject", nil, 404, nil)

	var full model.Task
	c.do("GET", path, nil, 200, &full)
	if len(full.Proposals) != 1 {
		t.Fatalf("GET task должен содержать отклики: %+v", full.Proposals)
	}

	// редактирование опубликованной снимает подтверждение и убирает из каталога до повторного confirm
	c.do("PUT", path+"/fields", map[string]any{"fields": map[string]string{"title": "Учёт заявок"}}, 200, &task)
	c.do("GET", "/api/tasks", nil, 200, &cat)
	if task.Confirmed || len(cat.Tasks) != 0 {
		t.Fatalf("после правки задача не должна быть подтверждённой в каталоге")
	}
}

func TestMiscEndpoints(t *testing.T) {
	c := testClient{t, NewHandler(newMemRepo(), fakeAI{}, fakeRating, t.TempDir()+"/missing")}
	var health map[string]any
	c.do("GET", "/api/health", nil, 200, &health)
	if health["ok"] != true || health["ai_mode"] != "mock" {
		t.Fatalf("health: %v", health)
	}
	var info ai.Info
	c.do("GET", "/api/ai", nil, 200, &info)
	if info.Mode != model.AIModeMock || info.PromptQuestions == "" {
		t.Fatalf("ai info: %+v", info)
	}
	var teams []model.Team
	c.do("GET", "/api/teams", nil, 200, &teams)
	if len(teams) != 1 {
		t.Fatalf("teams: %+v", teams)
	}
	c.do("GET", "/api/teams/1/recommended", nil, 200, &[]model.Task{})
	c.do("GET", "/api/teams/9/recommended", nil, 404, nil)
	c.do("GET", "/api/tasks/abc", nil, 400, nil)
	c.do("GET", "/api/nope", nil, 404, nil)

	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, httptest.NewRequest("GET", "/task/1", nil))
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("frontend not built")) {
		t.Fatalf("без сборки фронта ожидается подсказка, got %d %q", rec.Code, rec.Body.String())
	}
}

func TestSPAFallback(t *testing.T) {
	dir := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(writeFile(dir+"/index.html", "<html>app</html>"))
	must(writeFile(dir+"/app.js", "console.log(1)"))
	h := NewHandler(newMemRepo(), fakeAI{}, fakeRating, dir)
	for path, want := range map[string]string{"/": "app", "/task/5/edit": "app", "/app.js": "console.log"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Errorf("%s: %d %q", path, rec.Code, rec.Body.String())
		}
	}
}

func TestRecommend(t *testing.T) {
	team := model.Team{Skills: []string{"анализ данных"}, Tech: []string{"Python"}}
	tasks := []model.Task{
		{ID: 1, Rating: model.Rating{Score: 80}, Fields: model.Fields{model.FieldData: "Выгрузка данных продаж"}},
		{ID: 2, Rating: model.Rating{Score: 20}, Fields: model.Fields{model.FieldData: "данные"}},               // ниже порога 40
		{ID: 3, Rating: model.Rating{Score: 95}, Fields: model.Fields{model.FieldNeed: "мобильное приложение"}}, // нет пересечения
		{ID: 4, Rating: model.Rating{Score: 50}, Fields: model.Fields{model.FieldConstraints: "Python, анализ данных"}},
	}
	got := recommend(team, tasks, 5)
	if len(got) != 2 || got[0].ID != 4 || got[1].ID != 1 {
		ids := []int{}
		for _, g := range got {
			ids = append(ids, g.ID)
		}
		t.Fatalf("recommend: %v", ids)
	}
}
