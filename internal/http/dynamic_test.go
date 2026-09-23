package httpapi

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

type nextResp struct {
	Question     *model.Question  `json:"question"`
	Done         bool             `json:"done"`
	Reason       string           `json:"reason"`
	Asked        int              `json:"asked"`
	CardPreview  model.Fields     `json:"card_preview"`
	ScorePreview int              `json:"score_preview"`
	Missing      []model.FieldKey `json:"missing_fields"`
}

// Пошаговый режим: создание → 1 вопрос; next-question с ответами → done не раньше 3-го вопроса и не позже 5-го;
// POST /answers {} собирает карточку из сохранённых ответов.
func TestDynamicQuestions(t *testing.T) {
	repo := newMemRepo()
	c := testClient{t, NewHandler(repo, fakeAI{}, fakeRating, t.TempDir(), "")}
	biz := c.bizLogin("dyn@owner.kz")

	var task model.Task
	c.doAuth(biz, "POST", "/api/tasks", map[string]string{"draft_text": "Нужно приложение для учёта заявок", "industry": "IT", "mode": "dynamic"}, 201, &task)
	if len(task.Questions) != 1 || task.Questions[0].ID != 1 || task.Questions[0].InputType == "" {
		t.Fatalf("dynamic: ожидался 1 вопрос с input_type, got %+v", task.Questions)
	}
	var viaQuery model.Task
	c.doAuth(biz, "POST", "/api/tasks?mode=dynamic", map[string]string{"draft_text": "Нужно приложение для учёта заявок", "industry": "IT"}, 201, &viaQuery)
	if len(viaQuery.Questions) != 1 {
		t.Fatalf("?mode=dynamic: %d вопросов", len(viaQuery.Questions))
	}
	path := fmt.Sprintf("/api/tasks/%d", task.ID)

	answers := map[model.FieldKey]string{
		model.FieldContext:         "Сервисная компания, 40 заявок в день ведём в Excel",
		model.FieldData:            "Выгрузка заявок из Excel за год",
		model.FieldSuccessCriteria: "Ни одна заявка не теряется",
		model.FieldContact:         "dyn@owner.kz",
	}
	seen := map[model.FieldKey]bool{task.Questions[0].FieldKey: true}
	last := task.Questions[0]
	var r nextResp
	for i := 0; i < 6; i++ {
		c.doAuth(biz, "POST", path+"/next-question", map[string]string{"answer": answers[last.FieldKey]}, 200, &r)
		if r.Done {
			break
		}
		if r.Question == nil || seen[r.Question.FieldKey] {
			t.Fatalf("шаг %d: нет вопроса или повтор поля: %+v", i, r)
		}
		if r.Question.ID != r.Asked || r.Asked != i+2 {
			t.Fatalf("шаг %d: id=%d asked=%d", i, r.Question.ID, r.Asked)
		}
		seen[r.Question.FieldKey] = true
		last = *r.Question
	}
	if !r.Done || r.Asked < 3 || r.Asked > 5 || r.Reason == "" {
		t.Fatalf("ожидался done после 3–5 вопросов: %+v", r)
	}
	if r.CardPreview[model.FieldData] != answers[model.FieldData] || r.ScorePreview <= 10 {
		t.Fatalf("card_preview/score_preview: %+v", r)
	}
	stored := repo.tasks[task.ID]
	if len(stored.Questions) != r.Asked || stored.Questions[len(stored.Questions)-1].Answer == "" && answers[stored.Questions[len(stored.Questions)-1].FieldKey] != "" {
		t.Fatalf("ответ на последний вопрос не сохранён: %+v", stored.Questions)
	}
	// «спросить ещё» (без answer) после done — ещё вопрос; так до 8 всего, дальше done с причиной
	n := len(stored.Questions)
	for n < 8 {
		c.doAuth(biz, "POST", path+"/next-question", nil, 200, &r)
		n++
		if r.Done || r.Question == nil || r.Asked != n || len(repo.tasks[task.ID].Questions) != n {
			t.Fatalf("more после done, asked=%d: %+v", n, r)
		}
	}
	r = nextResp{}
	c.doAuth(biz, "POST", path+"/next-question", map[string]bool{"more": true}, 200, &r)
	if !r.Done || r.Reason == "" || r.Asked != 8 || len(repo.tasks[task.ID].Questions) != 8 {
		t.Fatalf("после 8 ожидался done: %+v", r)
	}

	// карточка из сохранённых ответов: {} и {answers:{}}
	for _, body := range []any{map[string]any{}, map[string]any{"answers": map[string]string{}}} {
		var card model.Task
		c.doAuth(biz, "POST", path+"/answers", body, 200, &card)
		if card.Status != model.StatusEditing || card.Fields[model.FieldData] != answers[model.FieldData] || card.Score == 0 {
			t.Fatalf("карточка из сохранённых ответов: %+v", card.Fields)
		}
	}
}

// Пропуск: пустой answer сохраняется как "", поле остаётся пробелом и переспрашивается (не больше 2 раз);
// done только после критичных полей и не позже 5 вопросов.
func TestDynamicSkipAndMax(t *testing.T) {
	repo := newMemRepo()
	c := testClient{t, NewHandler(repo, fakeAI{}, fakeRating, t.TempDir(), "")}
	biz := c.bizLogin("skip@owner.kz")
	var task model.Task
	c.doAuth(biz, "POST", "/api/tasks", map[string]string{"draft_text": "Нужно приложение", "industry": "IT", "mode": "dynamic"}, 201, &task)
	path := fmt.Sprintf("/api/tasks/%d/next-question", task.ID)
	count := map[model.FieldKey]int{task.Questions[0].FieldKey: 1}
	var r nextResp
	steps := 0
	for !r.Done {
		c.doAuth(biz, "POST", path, map[string]string{"answer": ""}, 200, &r)
		steps++
		if steps > 5 {
			t.Fatal("больше 5 вопросов")
		}
		if !r.Done {
			if count[r.Question.FieldKey]++; count[r.Question.FieldKey] > 2 {
				t.Fatalf("поле %s спрошено больше 2 раз", r.Question.FieldKey)
			}
		}
	}
	if r.Asked < 3 || r.Asked > 5 {
		t.Fatalf("asked=%d вне 3–5", r.Asked)
	}
}

// Шум («здравствуйте», «пока не знаю») не закрывает поле: оно остаётся в missing_fields и спрашивается
// ещё раз другой формулировкой; превью поле не заполняет.
func TestDynamicNoiseReasks(t *testing.T) {
	repo := newMemRepo()
	c := testClient{t, NewHandler(repo, fakeAI{}, fakeRating, t.TempDir(), "")}
	biz := c.bizLogin("noise@owner.kz")
	var task model.Task
	c.doAuth(biz, "POST", "/api/tasks", map[string]string{"draft_text": "фвфовфыов о", "industry": "education", "mode": "dynamic"}, 201, &task)
	first := task.Questions[0]
	path := fmt.Sprintf("/api/tasks/%d/next-question", task.ID)
	var r nextResp
	c.doAuth(biz, "POST", path, map[string]string{"answer": "здравствуйте"}, 200, &r)
	if r.Done || r.Question == nil || r.Question.FieldKey != first.FieldKey || r.Question.Text == first.Text {
		t.Fatalf("ожидалось переспрашивание %s иначе: %+v", first.FieldKey, r.Question)
	}
	if !slices.Contains(r.Missing, first.FieldKey) || strings.Contains(r.CardPreview[first.FieldKey], "здравствуйте") || r.CardPreview[model.FieldContext] != "" {
		t.Fatalf("поле после шума закрыто: missing=%v preview=%q", r.Missing, r.CardPreview[first.FieldKey])
	}
	c.doAuth(biz, "POST", path, map[string]string{"answer": "пока не знаю"}, 200, &r)
	if r.Done || r.Question == nil || r.Question.FieldKey == first.FieldKey {
		t.Fatalf("третий раз то же поле или done: %+v", r)
	}
}

// Права next-question — как у answers: без входа 401, чужой бизнес 403, задача без заявителя — 403.
func TestDynamicRequiresOwner(t *testing.T) {
	repo := newMemRepo()
	c := testClient{t, NewHandler(repo, fakeAI{}, fakeRating, t.TempDir(), "")}
	owner, other := c.bizLogin("own@owner.kz"), c.bizLogin("other@owner.kz")
	var task model.Task
	c.doAuth(owner, "POST", "/api/tasks", map[string]string{"draft_text": "Нужно приложение", "industry": "IT", "mode": "dynamic"}, 201, &task)
	path := fmt.Sprintf("/api/tasks/%d/next-question", task.ID)
	body := map[string]string{"answer": "x"}
	c.do("POST", path, body, 401, nil)
	c.doAuth(other, "POST", path, body, 403, nil)
	c.doAuth(owner, "POST", path, "{broken", 400, nil)
	c.doAuth(owner, "POST", "/api/tasks/999/next-question", body, 404, nil)
	if repo.tasks[task.ID].Questions[0].Answer != "" {
		t.Fatal("чужой запрос записал ответ")
	}
	seed := model.Task{Industry: "IT", Status: model.StatusClarifying, Fields: model.Fields{}.Full()}
	if err := repo.CreateTask(t.Context(), &seed); err != nil {
		t.Fatal(err)
	}
	c.doAuth(owner, "POST", fmt.Sprintf("/api/tasks/%d/next-question", seed.ID), body, 403, nil)
}
