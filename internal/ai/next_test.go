package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/rating"
)

// Mock пошагового режима: 3–5 вопросов без повторов полей, done после критичных (данные, критерии, контакт).
func TestMockNextQuestionSequence(t *testing.T) {
	c := New("", "")
	answers := map[model.FieldKey]string{
		model.FieldContext:         "Сервисная компания, заявки в Excel",
		model.FieldData:            "Выгрузка заявок из Excel",
		model.FieldSuccessCriteria: "Ни одна заявка не теряется",
		model.FieldContact:         "ivan@firm.kz",
	}
	var asked []model.Question
	for {
		r, err := c.NextQuestion(context.Background(), weakDraft, "Логистика", asked, false)
		if err != nil {
			t.Fatal(err)
		}
		if r.Done {
			if len(asked) < MinDynamicQuestions || r.Reason == "" || r.Question != nil {
				t.Fatalf("done после %d вопросов: %+v", len(asked), r)
			}
			break
		}
		q := r.Question
		if q == nil || q.Text == "" || !validKey(q.FieldKey) || q.InputType == "" {
			t.Fatalf("плохой вопрос: %+v", q)
		}
		for _, a := range asked {
			if a.FieldKey == q.FieldKey && !rating.IsNoise(a.FieldKey, a.Answer) {
				t.Fatalf("повтор поля %s", q.FieldKey)
			}
		}
		q.Answer = answers[q.FieldKey]
		asked = append(asked, *q)
		if len(asked) > MaxDynamicQuestions {
			t.Fatal("больше 5 вопросов")
		}
	}
	got := map[model.FieldKey]bool{}
	for _, q := range asked {
		got[q.FieldKey] = true
	}
	for _, k := range []model.FieldKey{model.FieldData, model.FieldSuccessCriteria, model.FieldContact} {
		if !got[k] {
			t.Errorf("критичное поле %s не спрошено", k)
		}
	}
	if asked[0].FieldKey != model.FieldContext {
		t.Errorf("первым ожидался context, got %s", asked[0].FieldKey)
	}
}

// Поле, закрытое ответом на другой вопрос, не спрашивается.
func TestMockNextQuestionSkipsFieldsClosedByAnswers(t *testing.T) {
	asked := []model.Question{{ID: 1, FieldKey: model.FieldContext, Answer: "Склад, данные в CRM, контакт ivan@firm.kz"}}
	for len(asked) < MaxDynamicQuestions {
		r, _ := New("", "").NextQuestion(context.Background(), weakDraft, "", asked, false)
		if r.Done {
			break
		}
		if r.Question.FieldKey == model.FieldData || r.Question.FieldKey == model.FieldContact {
			t.Fatalf("спрошено поле, закрытое ответом: %s", r.Question.FieldKey)
		}
		asked = append(asked, *r.Question)
	}
}

// Сильный черновик: всё раскрыто, но до 3 вопросов done не бывает; на 5 — всегда done.
func TestNextQuestionMinMax(t *testing.T) {
	draft := "Мы логистическая компания, нужно приложение для клиентов. Данные из CRM, срок 2 месяца, " +
		"результат — прототип, метрика — рост заявок на 10%. Контакт: ivan@firm.kz, созвон раз в неделю."
	c := New("", "")
	var asked []model.Question
	for i := 0; i < MinDynamicQuestions; i++ {
		r, _ := c.NextQuestion(context.Background(), draft, "", asked, false)
		if r.Done || r.Question == nil {
			t.Fatalf("done при asked=%d", i)
		}
		asked = append(asked, *r.Question)
	}
	five := make([]model.Question, MaxDynamicQuestions)
	r, _ := c.NextQuestion(context.Background(), weakDraft, "", five, false)
	if !r.Done || r.Question != nil {
		t.Fatalf("при 5 вопросах ожидался done: %+v", r)
	}
}

// OpenAI вернул done=true при asked < 3 — правило поверх модели всё равно даёт вопрос (по первому missing).
func TestOpenAINextDoneTooEarly(t *testing.T) {
	var sys string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req responsesRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Text.Format.Name != "next_question" || len(req.Input) != 2 {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		sys = req.Input[0].Content
		w.Write(responsesBody(t, map[string]any{
			"done": true, "reason": "всё понятно", "question": nil,
			"missing_fields": []string{"data", "context", "users"},
		}))
	}))
	defer srv.Close()
	c := newClient("k", "", srv.URL)
	asked := []model.Question{{ID: 1, FieldKey: model.FieldContext, Text: "?", Answer: "Склад"}}
	r, err := c.NextQuestion(context.Background(), weakDraft, "", asked, false)
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode() != model.AIModeOpenAI || sys != PromptNextQuestion {
		t.Fatalf("mode=%s, промпт не тот", c.Mode())
	}
	if r.Done || r.Question == nil || r.Question.FieldKey != model.FieldData || r.Question.InputType == "" {
		t.Fatalf("ожидался вопрос про data: %+v", r)
	}
	for _, k := range r.MissingFields {
		if k == model.FieldContext {
			t.Error("уже спрошенное поле осталось в missing_fields")
		}
	}
}

// OpenAI: вопрос модели проходит с input_type/suggestions; повтор поля заменяется; при asked ≥ 3 done принимается.
func TestOpenAINextQuestionRules(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(responsesBody(t, payload))
	}))
	defer srv.Close()
	c := newClient("k", "", srv.URL)
	asked := []model.Question{{ID: 1, FieldKey: model.FieldContext, Answer: "Склад, заявки в Excel"}}

	payload = map[string]any{"done": false, "reason": "", "missing_fields": []string{"users"}, "question": map[string]any{
		"text": "Кто будет пользоваться? (менеджеры)", "field_key": "users", "input_type": "multi",
		"suggestions": []string{"Менеджеры", " ", "Менеджеры", "Клиенты"}}}
	r, _ := c.NextQuestion(context.Background(), weakDraft, "", asked, false)
	if r.Question == nil || r.Question.InputType != model.InputMulti || strings.Join(r.Question.Suggestions, "|") != "Менеджеры|Клиенты" {
		t.Fatalf("вопрос модели: %+v", r.Question)
	}

	payload["question"] = map[string]any{"text": "Опять контекст?", "field_key": "context", "input_type": "bogus", "suggestions": []string{}}
	r, _ = c.NextQuestion(context.Background(), weakDraft, "", asked, false)
	if r.Question == nil || r.Question.FieldKey != model.FieldUsers {
		t.Fatalf("повтор поля не заменён: %+v", r.Question)
	}

	three := []model.Question{{FieldKey: model.FieldContext, Answer: "Склад"}, {FieldKey: model.FieldData, Answer: "Выгрузка из CRM"}, {FieldKey: model.FieldContact, Answer: "ivan@firm.kz"}}
	payload = map[string]any{"done": true, "reason": "достаточно", "question": nil, "missing_fields": []string{}}
	r, _ = c.NextQuestion(context.Background(), weakDraft, "", three, false)
	if !r.Done || r.Reason != "достаточно" || r.Question != nil {
		t.Fatalf("done при asked=3 не принят: %+v", r)
	}
}

func TestNextQuestionSchemaStrict(t *testing.T) {
	b, _ := json.Marshal(nextQuestionSchema())
	for _, want := range []string{`"null"`, `"input_type"`, `"suggestions"`, `"yes_no"`, `"additionalProperties":false`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("схема без %s", want)
		}
	}
}

// Шум не закрывает поле: остаётся в missing_fields, переспрашивается другой формулировкой, не больше 2 раз.
func TestNextQuestionNoiseReasks(t *testing.T) {
	c := New("", "")
	r, _ := c.NextQuestion(context.Background(), weakDraft, "", nil, false)
	first := *r.Question
	first.Answer = "здравствуйте"
	asked := []model.Question{first}
	r, _ = c.NextQuestion(context.Background(), weakDraft, "", asked, false)
	if r.Question == nil || r.Question.FieldKey != first.FieldKey || r.Question.Text == first.Text {
		t.Fatalf("ожидался повтор %s другой формулировкой: %+v", first.FieldKey, r.Question)
	}
	if !slices.Contains(r.MissingFields, first.FieldKey) {
		t.Fatalf("поле после шума пропало из missing: %v", r.MissingFields)
	}
	second := *r.Question
	second.Answer = "не знаю"
	r, _ = c.NextQuestion(context.Background(), weakDraft, "", append(asked, second), false)
	if r.Question == nil || r.Question.FieldKey == first.FieldKey {
		t.Fatalf("поле спрошено третий раз: %+v", r.Question)
	}
}

// more после done: следующий вопрос вплоть до MaxExtraQuestions, дальше done с причиной.
func TestNextQuestionMore(t *testing.T) {
	c := New("", "")
	var asked []model.Question
	for {
		r, _ := c.NextQuestion(context.Background(), weakDraft, "", asked, false)
		if r.Done {
			break
		}
		r.Question.Answer = "Ответ по делу про " + string(r.Question.FieldKey)
		asked = append(asked, *r.Question)
	}
	for len(asked) < MaxExtraQuestions {
		r, _ := c.NextQuestion(context.Background(), weakDraft, "", asked, true)
		if r.Done || r.Question == nil {
			t.Fatalf("more при asked=%d не дал вопрос: %+v", len(asked), r)
		}
		r.Question.Answer = "Ответ по делу"
		asked = append(asked, *r.Question)
	}
	r, _ := c.NextQuestion(context.Background(), weakDraft, "", asked, true)
	if !r.Done || r.Question != nil || r.Reason == "" {
		t.Fatalf("после %d ожидался done: %+v", MaxExtraQuestions, r)
	}
}
