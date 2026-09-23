package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

func TestDiscoveryPreservesMeaningfulFollowups(t *testing.T) {
	for _, tc := range []struct {
		name  string
		draft string
		asked []model.Question
		field model.FieldKey
		text  string
	}{
		{"long vague draft", strings.Repeat("Хочу хороший современный проект для своего бизнеса. ", 3), nil, model.FieldContext, "Какой работой занимается бизнес и в каком процессе нужна помощь?"},
		{"partial answer", "Нужна система для заявок", []model.Question{{FieldKey: model.FieldNeed, Text: "Что мешает работе?", Answer: "Теряем клиентов"}}, model.FieldNeed, "На каком шаге клиент обычно пропадает: до первого ответа или после встречи?"},
		{"after fifth question", "Нужна система для заявок", []model.Question{{FieldKey: model.FieldContext, Answer: "Курсы"}, {FieldKey: model.FieldUsers, Answer: "Администратор"}, {FieldKey: model.FieldData, Answer: "Таблица"}, {FieldKey: model.FieldConstraints, Answer: "Без бота"}, {FieldKey: model.FieldNeed, Answer: "Не возвращаются после пробного занятия"}}, model.FieldExpectedResult, "Начать можно с напоминаний администратору. Какое действие он должен делать после пробного занятия?"},
		{"no data is useful", "Запускаем новый проект", []model.Question{{FieldKey: model.FieldData, Answer: "Нет данных"}}, model.FieldExpectedResult, "Тогда проверим идею без готовых данных. Какой сценарий вы могли бы показать первым пользователям?"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write(responsesBody(t, map[string]any{"done": false, "reason": "", "missing_fields": []model.FieldKey{tc.field}, "question": map[string]any{"field_key": tc.field, "text": tc.text, "input_type": "text", "suggestions": []string{}}}))
			}))
			defer srv.Close()
			c := newClient("test", "", srv.URL)
			r, err := c.NextQuestion(context.Background(), tc.draft, "", tc.asked, false)
			if err != nil || r.Done || r.Question == nil || r.Question.Text != tc.text || r.Question.FieldKey != tc.field {
				t.Fatalf("meaningful model question was replaced: %+v, %v", r, err)
			}
		})
	}
}

func TestDiscoverySkipAndCompletion(t *testing.T) {
	asks := []model.Question{{FieldKey: model.FieldContext, Answer: "Курсы"}, {FieldKey: model.FieldNeed, Answer: "Теряем заявки"}, {FieldKey: model.FieldData, Answer: ""}}
	r := finishNext(model.NextQuestionResult{Question: &model.Question{FieldKey: model.FieldData, Text: "Снова про данные?"}, MissingFields: []model.FieldKey{model.FieldData, model.FieldSuccessCriteria}}, asks, "Нужна помощь", false)
	if r.Question == nil || r.Question.FieldKey == model.FieldData {
		t.Fatalf("explicit skip was ignored: %+v", r)
	}
	r = finishNext(model.NextQuestionResult{Done: true, Reason: "Понятны задача и первый полезный сценарий", MissingFields: []model.FieldKey{model.FieldContact}}, asks, "Нужна помощь", false)
	if !r.Done {
		t.Fatalf("contact must not block a useful solution: %+v", r)
	}
	asks[2].Answer = "Нет данных"
	r = finishNext(model.NextQuestionResult{Done: true, Reason: "Сначала проверим идею без материалов"}, asks, "Нужна помощь", false)
	if !r.Done {
		t.Fatalf("absence of materials is a useful answer, not a reason to repeat: %+v", r)
	}
	r = finishNext(model.NextQuestionResult{}, make([]model.Question, MaxDynamicQuestions), "Нужна помощь", false)
	if !r.Done || strings.Contains(r.Reason, "достаточно") {
		t.Fatalf("safety cap claimed understanding: %+v", r)
	}
}

// Opt-in live smoke: synthetic projects only, no tasks or database writes.
// AI_DISCOVERY_LIVE=1 OPENAI_API_KEY=... go test ./internal/ai -run TestDiscoveryLive -v
func TestDiscoveryLive(t *testing.T) {
	if os.Getenv("AI_DISCOVERY_LIVE") == "" || os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("opt-in live check")
	}
	c := New(os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_MODEL"))
	for _, tc := range []struct {
		name, draft string
		asked       []model.Question
	}{
		{"vague", "Хочу сделать современный и удобный проект для своего бизнеса, чтобы всё работало лучше и клиентам нравилось, но не понимаю с чего начать.", nil},
		{"specific", "У нас языковые курсы. Заявки из WhatsApp ведёт администратор в таблице, часть людей теряется после пробного занятия.", []model.Question{{FieldKey: model.FieldNeed, Text: "На каком шаге теряется клиент?", Answer: "После пробного администратор забывает написать и спросить решение. Нужен способ не забывать."}}},
		{"rejected approach", "Хотели бота для записи на курсы.", []model.Question{{FieldKey: model.FieldContext, Text: "Как записываются сейчас?", Answer: "Администратор вручную отвечает в переписке"}, {FieldKey: model.FieldNeed, Text: "Что нужно улучшить?", Answer: "Передумал, бот не нужен. Клиенты хотят живого человека. Нужны напоминания администратору, чтобы он не забывал связаться после пробного."}, {FieldKey: model.FieldUsers, Text: "Кто будет работать?", Answer: "Только администратор, клиенты ничего нового устанавливать не должны"}}},
	} {
		r, err := c.NextQuestion(t.Context(), tc.draft, "Образование", tc.asked, false)
		if err != nil || c.Mode() != model.AIModeOpenAI {
			t.Fatalf("%s: real model unavailable: %v", tc.name, err)
		}
		b, _ := json.Marshal(r)
		t.Logf("%s: %s", tc.name, b)
		if r.Question == nil || r.Done {
			t.Errorf("%s: expected useful clarification", tc.name)
		}
		if r.Question != nil && r.Question.FieldKey == model.FieldContact {
			t.Errorf("%s: asked contact before understanding", tc.name)
		}
	}
	r, err := c.ResultOptions(t.Context(), "Языковые курсы теряют клиентов после пробного занятия: администратор забывает связаться.", "Образование", []model.Question{
		{FieldKey: model.FieldExpectedResult, Text: "Как помочь администратору?", Answer: "Нужны напоминания о клиентах после пробного. Бот не нужен, клиенты общаются с человеком и ничего не устанавливают."},
		{FieldKey: model.FieldData, Text: "Что есть сейчас?", Answer: "Таблица с контактами и датой пробного занятия."},
		{FieldKey: model.FieldSuccessCriteria, Text: "Как проверим?", Answer: "Администратор видит, кому сегодня написать, отмечает результат и не теряет клиента."},
	})
	if err != nil || c.Mode() != model.AIModeOpenAI || len(r.Options) < 2 {
		t.Fatalf("real solution options unavailable: %+v, %v", r, err)
	}
	b, _ := json.Marshal(r)
	t.Logf("solution options: %s", b)
}
