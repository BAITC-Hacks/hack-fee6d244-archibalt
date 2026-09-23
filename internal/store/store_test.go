package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// Интеграционный тест: нужен Postgres, например
//
//	TEST_DATABASE_URL=postgres://postgres:hack@localhost:5432/hack?sslmode=disable go test ./internal/store/
//
// Работает в отдельной временной схеме (search_path), схема удаляется после теста.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL не задан — пропускаю интеграционный тест store")
	}
	ctx := context.Background()
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())

	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	st, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func testRating(f model.Fields, _ bool) model.Rating {
	score := 0
	for _, v := range f {
		if v != "" {
			score += 10
		}
	}
	return model.Rating{Score: score, Level: model.LevelDraft, LevelLabel: "черновик", Breakdown: []model.BreakdownItem{}, Missing: []model.MissingItem{}}
}

func TestMigrateSeedAndQueries(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	for i := 0; i < 2; i++ { // миграция идемпотентна
		if err := st.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SeedIfEmpty(ctx, "testdata", testRating); err != nil {
		t.Fatal(err)
	}
	if err := st.SeedIfEmpty(ctx, "testdata", testRating); err != nil { // второй раз — пропуск
		t.Fatal(err)
	}

	tasks, industries, err := st.ListTasks(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].ID != 1 || tasks[0].Score != 30 || len(industries) != 1 {
		t.Fatalf("каталог после seed: %d задач, %v", len(tasks), industries)
	}
	if tasks[0].PublishedAt == nil || len(tasks[0].Fields) != 10 || tasks[0].Questions[0].ID != 1 {
		t.Fatalf("seed-задача: %+v", tasks[0])
	}
	if got, _, _ := st.ListTasks(ctx, "Образование", ""); len(got) != 0 {
		t.Fatal("неподтверждённая задача не должна быть в каталоге")
	}
	if got, _, _ := st.ListTasks(ctx, "", "draft"); len(got) != 2 {
		t.Fatalf("фильтр level=draft: %d", len(got))
	}

	// последовательности сдвинуты за max(id): новая задача получает id 6
	nt := model.Task{Industry: "IT", Status: model.StatusClarifying, DraftText: "x", Fields: model.Fields{}, Rating: testRating(nil, false),
		Questions: []model.Question{{ID: 1, Text: "q", FieldKey: model.FieldUsers}}, AIMode: model.AIModeMock}
	if err := st.CreateTask(ctx, &nt); err != nil {
		t.Fatal(err)
	}
	if nt.ID != 6 {
		t.Fatalf("новая задача id=%d, ожидалось 6", nt.ID)
	}
	nt.Fields[model.FieldTitle] = "Заголовок"
	nt.Rating = testRating(nt.Fields, true)
	now := time.Now()
	nt.Status, nt.Confirmed, nt.PublishedAt = model.StatusPublished, true, &now
	if err := st.UpdateTask(ctx, &nt); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetTask(ctx, nt.ID)
	if err != nil || got.Fields[model.FieldTitle] != "Заголовок" || !got.Confirmed || got.Score != 10 {
		t.Fatalf("GetTask после update: %+v, %v", got, err)
	}
	if _, err := st.GetTask(ctx, 999); err != ErrNotFound {
		t.Fatalf("ожидался ErrNotFound, got %v", err)
	}

	withProps, _ := st.GetTask(ctx, 1)
	if len(withProps.Proposals) != 1 || withProps.Proposals[0].Team.Name != "Байты" {
		t.Fatalf("отклики задачи 1: %+v", withProps.Proposals)
	}

	p := model.Proposal{TaskID: nt.ID, Team: model.TeamRef{ID: 2}, Idea: "i", Plan: "p", Deadline: "d", Link: "l"}
	if err := st.CreateProposal(ctx, &p); err != nil {
		t.Fatal(err)
	}
	if p.ID != 5 || p.Team.Name != "Пиксели" || p.Status != model.ProposalNew {
		t.Fatalf("новый отклик: %+v", p)
	}
	bad := model.Proposal{TaskID: nt.ID, Team: model.TeamRef{ID: 99}, Idea: "i", Plan: "p", Deadline: "d", Link: "l"}
	if err := st.CreateProposal(ctx, &bad); err != ErrNotFound {
		t.Fatalf("FK-нарушение должно давать ErrNotFound, got %v", err)
	}
	if p, err = st.UpdateProposalStatus(ctx, p.ID, model.ProposalSelected); err != nil || p.Status != model.ProposalSelected {
		t.Fatalf("select: %+v %v", p, err)
	}
	for i := 0; i < 2; i++ {
		if p, err = st.ConfirmStage(ctx, p.ID); err != nil || !p.StageConfirmed {
			t.Fatalf("confirm stage: %+v %v", p, err)
		}
	}
	team, err := st.GetTeam(ctx, 2)
	if err != nil || team.Points != 5+StagePoints {
		t.Fatalf("баллы команды: %+v %v", team, err)
	}
	if _, err := st.ConfirmStage(ctx, 999); err != ErrNotFound {
		t.Fatalf("confirm stage несуществующего: %v", err)
	}
	teams, err := st.ListTeams(ctx)
	if err != nil || len(teams) != 2 || teams[0].ID != 2 {
		t.Fatalf("ListTeams: %+v %v", teams, err)
	}
}

func TestSeedFKValidation(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("teams.json", `[{"id":1,"name":"A","skills":[],"interests":[],"tech":[]}]`)
	write("tasks.json", `[{"id":1,"industry":"IT","draft_text":"x","fields":{},"confirmed":true}]`)
	write("proposals.json", `[{"id":1,"task_id":1,"team_id":7,"idea":"i","plan":"p","deadline":"d","link":"l"}]`)
	if err := st.SeedIfEmpty(ctx, dir, testRating); err == nil {
		t.Fatal("ожидалась ошибка FK team_id 7")
	}
	if n, _, _ := st.ListTasks(ctx, "", ""); len(n) != 0 {
		t.Fatal("при ошибке seed ничего не должно загрузиться")
	}
}
