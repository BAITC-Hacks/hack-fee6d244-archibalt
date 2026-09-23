package httpapi

import (
	"fmt"
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

type optionsResp struct {
	Options []model.ResultOption `json:"options"`
	AIMode  model.AIMode         `json:"ai_mode"`
}

// Варианты первого результата: владелец получает 2–3 варианта; apply-result переносит выбранный
// (с правкой check) в expected_result / success_criteria, снимает подтверждение, балл растёт.
func TestResultOptionsApply(t *testing.T) {
	repo := newMemRepo()
	c := testClient{t, NewHandler(repo, fakeAI{}, fakeRating, t.TempDir(), "")}
	biz := c.bizLogin("opts@owner.kz")
	var task model.Task
	c.doAuth(biz, "POST", "/api/tasks", map[string]string{"draft_text": "Нужно приложение для учёта заявок", "industry": "IT", "mode": "dynamic"}, 201, &task)
	path := fmt.Sprintf("/api/tasks/%d", task.ID)
	c.doAuth(biz, "POST", path+"/answers", map[string]any{"answers": map[string]string{"1": "Выгрузка заявок из Excel"}}, 200, &task)

	var or optionsResp
	c.doAuth(biz, "POST", path+"/result-options", nil, 200, &or)
	if len(or.Options) < 2 || len(or.Options) > 3 || or.AIMode != model.AIModeMock {
		t.Fatalf("result-options: %+v", or)
	}
	for _, o := range or.Options {
		if o.Title == "" || o.Result == "" || o.Check == "" || o.Needs == "" || o.Weeks < 2 || o.Weeks > 8 {
			t.Fatalf("плохой вариант: %+v", o)
		}
	}
	if repo.tasks[task.ID].Fields[model.FieldExpectedResult] != "" {
		t.Fatal("result-options не должен менять задачу")
	}

	c.doAuth(biz, "POST", path+"/confirm", nil, 200, &task)
	before := task.Score
	var got model.Task
	c.doAuth(biz, "POST", path+"/apply-result", map[string]any{"index": 1, "edits": map[string]string{"check": "  Менеджер находит заявку за минуту  "}}, 200, &got)
	if got.Fields[model.FieldExpectedResult] != or.Options[1].Result || got.Fields[model.FieldSuccessCriteria] != "Менеджер находит заявку за минуту" {
		t.Fatalf("поля не перенесены: %+v", got.Fields)
	}
	if got.Confirmed || got.PreviousScore == nil || *got.PreviousScore != before || got.Score <= before {
		t.Fatalf("confirmed=%v previous=%v score %d → %d", got.Confirmed, got.PreviousScore, before, got.Score)
	}

	c.doAuth(biz, "POST", path+"/apply-result", map[string]any{"index": 5}, 400, nil)
	c.doAuth(biz, "POST", path+"/apply-result", map[string]any{}, 400, nil)
	c.doAuth(biz, "POST", path+"/apply-result", "{broken", 400, nil)
}

// Права — как у answers: без входа 401, чужой бизнес 403, задача без заявителя — 403.
func TestResultOptionsRequireOwner(t *testing.T) {
	repo := newMemRepo()
	c := testClient{t, NewHandler(repo, fakeAI{}, fakeRating, t.TempDir(), "")}
	owner, other := c.bizLogin("own@owner.kz"), c.bizLogin("other@owner.kz")
	var task model.Task
	c.doAuth(owner, "POST", "/api/tasks", map[string]string{"draft_text": "Нужно приложение", "industry": "IT", "mode": "dynamic"}, 201, &task)
	seed := model.Task{Industry: "IT", Status: model.StatusPublished, Fields: model.Fields{}.Full()}
	if err := repo.CreateTask(t.Context(), &seed); err != nil {
		t.Fatal(err)
	}
	apply := map[string]any{"index": 0}
	for _, p := range []struct {
		suffix string
		body   any
	}{{"/result-options", nil}, {"/apply-result", apply}} {
		path := fmt.Sprintf("/api/tasks/%d%s", task.ID, p.suffix)
		c.do("POST", path, p.body, 401, nil)
		c.doAuth(other, "POST", path, p.body, 403, nil)
		c.doAuth(owner, "POST", fmt.Sprintf("/api/tasks/%d%s", seed.ID, p.suffix), p.body, 403, nil)
		c.doAuth(owner, "POST", "/api/tasks/999"+p.suffix, p.body, 404, nil)
	}
	if repo.tasks[task.ID].Fields[model.FieldExpectedResult] != "" {
		t.Fatal("чужой запрос изменил задачу")
	}
	// apply-result без предварительного result-options — варианты пересчитываются
	c.doAuth(owner, "POST", fmt.Sprintf("/api/tasks/%d/apply-result", task.ID), apply, 200, nil)
}
