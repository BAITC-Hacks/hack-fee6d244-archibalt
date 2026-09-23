package httpapi

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/ai"
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

// Пошаговый режим: создание → 1 вопрос; next-question с ответами → done в пределах лимитов AI;
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
	for i := 0; i < ai.MaxDynamicQuestions; i++ {
		r = nextResp{}
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
	if !r.Done || r.Question != nil || r.Asked < ai.MinDynamicQuestions || r.Asked > ai.MaxDynamicQuestions || r.Reason == "" {
		t.Fatalf("ожидался done после %d–%d вопросов: %+v", ai.MinDynamicQuestions, ai.MaxDynamicQuestions, r)
	}
	if r.CardPreview[model.FieldData] != answers[model.FieldData] || r.ScorePreview <= 10 {
		t.Fatalf("card_preview/score_preview: %+v", r)
	}
	stored := repo.tasks[task.ID]
	if len(stored.Questions) != r.Asked || stored.Questions[len(stored.Questions)-1].Answer == "" && answers[stored.Questions[len(stored.Questions)-1].FieldKey] != "" {
		t.Fatalf("ответ на последний вопрос не сохранён: %+v", stored.Questions)
	}
	// «спросить ещё» (без answer) продолжает по оставшимся темам: лимит — верхняя граница,
	// а не обязательное число вопросов; когда уточнять нечего, допустим ранний done.
	n := len(stored.Questions)
	for {
		r = nextResp{}
		c.doAuth(biz, "POST", path+"/next-question", nil, 200, &r)
		if r.Done {
			break
		}
		n++
		if n > ai.MaxExtraQuestions || r.Question == nil || r.Question.ID != n || r.Asked != n || len(repo.tasks[task.ID].Questions) != n {
			t.Fatalf("more после done, asked=%d: %+v", n, r)
		}
	}
	if r.Question != nil || r.Reason == "" || r.Asked != n || len(repo.tasks[task.ID].Questions) != n || n > ai.MaxExtraQuestions {
		t.Fatalf("ожидался done не позже %d вопросов: %+v", ai.MaxExtraQuestions, r)
	}
	r = nextResp{}
	c.doAuth(biz, "POST", path+"/next-question", map[string]bool{"more": true}, 200, &r)
	if !r.Done || r.Question != nil || r.Reason == "" || r.Asked != n || len(repo.tasks[task.ID].Questions) != n {
		t.Fatalf("после завершения оставшихся тем ожидался done: %+v", r)
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

// Явный пропуск: пустой answer сохраняется как "", тему не переспрашиваем;
// done — не раньше минимума и не позже верхней границы вопросов.
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
		r = nextResp{}
		c.doAuth(biz, "POST", path, map[string]string{"answer": ""}, 200, &r)
		steps++
		if steps > ai.MaxDynamicQuestions {
			t.Fatalf("больше %d вопросов", ai.MaxDynamicQuestions)
		}
		if !r.Done {
			if r.Question == nil || r.Question.ID != r.Asked || r.Asked != steps+1 {
				t.Fatalf("нарушен порядок вопросов после пропуска: %+v", r)
			}
			if count[r.Question.FieldKey]++; count[r.Question.FieldKey] > 1 {
				t.Fatalf("пропущенное поле %s спрошено повторно", r.Question.FieldKey)
			}
		}
	}
	if r.Question != nil || r.Reason == "" || r.Asked < ai.MinDynamicQuestions || r.Asked > ai.MaxDynamicQuestions || len(repo.tasks[task.ID].Questions) != r.Asked {
		t.Fatalf("done вне границ %d–%d: %+v", ai.MinDynamicQuestions, ai.MaxDynamicQuestions, r)
	}
	for _, q := range repo.tasks[task.ID].Questions {
		if q.Answer != "" {
			t.Fatalf("пропуск сохранён как непустой ответ: %+v", q)
		}
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
	// анонимный черновик (без заявителя, clarifying): вопросы доступны без входа; опубликованная задача без заявителя — 403
	anon := model.Task{Industry: "IT", Status: model.StatusClarifying, Fields: model.Fields{}.Full()}
	if err := repo.CreateTask(t.Context(), &anon); err != nil {
		t.Fatal(err)
	}
	c.do("POST", fmt.Sprintf("/api/tasks/%d/next-question", anon.ID), body, 200, nil)
	seed := model.Task{Industry: "IT", Status: model.StatusPublished, Fields: model.Fields{}.Full()}
	if err := repo.CreateTask(t.Context(), &seed); err != nil {
		t.Fatal(err)
	}
	c.doAuth(owner, "POST", fmt.Sprintf("/api/tasks/%d/next-question", seed.ID), body, 403, nil)
}
