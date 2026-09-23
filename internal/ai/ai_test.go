package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

const weakDraft = "Нужно приложение для учёта заявок"

func checkQuestions(t *testing.T, r model.QuestionsResult) {
	t.Helper()
	if len(r.Questions) < 3 || len(r.Questions) > 5 {
		t.Fatalf("ожидалось 3–5 вопросов, получено %d: %+v", len(r.Questions), r.Questions)
	}
	ids := map[int]bool{}
	for _, q := range r.Questions {
		if !validKey(q.FieldKey) {
			t.Errorf("невалидный field_key %q", q.FieldKey)
		}
		if ids[q.ID] || q.ID <= 0 {
			t.Errorf("id %d не уникален или ≤ 0", q.ID)
		}
		ids[q.ID] = true
		if q.Text == "" || q.Answer != "" {
			t.Errorf("плохой вопрос %+v", q)
		}
	}
}

func TestMockQuestionsWeakDraft(t *testing.T) {
	c := New("", "")
	r, err := c.Questions(context.Background(), weakDraft, "Логистика")
	if err != nil {
		t.Fatal(err)
	}
	checkQuestions(t, r)
	if c.Mode() != model.AIModeMock {
		t.Errorf("mode = %s", c.Mode())
	}
	if r.Questions[0].FieldKey != model.FieldContext {
		t.Errorf("первым ожидался вопрос про context, получен %s", r.Questions[0].FieldKey)
	}
	for _, k := range r.MissingFields {
		if k == model.FieldNeed || k == model.FieldExpectedResult {
			t.Errorf("поле %s раскрыто в черновике, но попало в missing", k)
		}
	}
}

func TestMockQuestionsFullDraftStillThree(t *testing.T) {
	draft := "Мы логистическая компания, нужно приложение для клиентов. Данные из CRM, срок 2 месяца, " +
		"результат — прототип, метрика — рост заявок на 10%. Контакт: ivan@firm.kz, созвон раз в неделю."
	r, _ := New("", "").Questions(context.Background(), draft, "Логистика")
	checkQuestions(t, r)
}

func TestMockCardFromAnswers(t *testing.T) {
	qs := []model.Question{
		{ID: 1, FieldKey: model.FieldData, Answer: "Выгрузка заявок из Excel за последний год"},
		{ID: 2, FieldKey: model.FieldUsers, Answer: "Диспетчеры склада и менеджеры по продажам"},
		{ID: 3, FieldKey: model.FieldSuccessCriteria, Answer: "Время обработки заявки сократится до 10 минут"},
		{ID: 4, FieldKey: model.FieldContact, Answer: ""},
	}
	c := New("", "")
	r, err := c.Card(context.Background(), weakDraft, "Логистика", qs)
	if err != nil {
		t.Fatal(err)
	}
	f := r.Fields
	if len(f) != len(model.FieldKeys) {
		t.Errorf("ожидалось %d ключей, получено %d", len(model.FieldKeys), len(f))
	}
	if f[model.FieldTitle] == "" {
		t.Error("пустой title")
	}
	for _, q := range qs[:3] {
		if f[q.FieldKey] != q.Answer {
			t.Errorf("поле %s = %q, ожидалось %q (guard выкинул честный ответ?)", q.FieldKey, f[q.FieldKey], q.Answer)
		}
	}
	if f[model.FieldContext] != weakDraft {
		t.Errorf("context = %q", f[model.FieldContext])
	}
	if f[model.FieldContact] != "" || f[model.FieldConstraints] != "" {
		t.Errorf("незаполненные поля должны быть пустыми: %+v", f)
	}
}

func TestGuardDropsInventedFacts(t *testing.T) {
	src := weakDraft + "\nВыгрузка заявок из Excel"
	f := Guard(model.Fields{
		model.FieldTitle:       "Учёт заявок",
		model.FieldConstraints: "Бюджет 5 млн тенге, срок 3 месяца",
		model.FieldData:        "Выгрузка заявок из Excel",
		model.FieldNeed:        "Нужно приложение для учёта заявок в 2 раза быстрее", // выдуманное число
	}, src)
	if f[model.FieldConstraints] != "" {
		t.Errorf("выдуманные ограничения не удалены: %q", f[model.FieldConstraints])
	}
	if f[model.FieldNeed] != "" {
		t.Errorf("выдуманное число не удалено: %q", f[model.FieldNeed])
	}
	if f[model.FieldData] == "" || f[model.FieldTitle] == "" {
		t.Errorf("честные поля удалены: %+v", f)
	}
}

func TestGuardKeepsCompressedWording(t *testing.T) {
	src := "Нашим менеджерам нужно быстрее обрабатывать входящие заявки клиентов, сейчас всё в таблицах"
	f := Guard(model.Fields{model.FieldNeed: "Менеджерам нужно быстрее обрабатывать заявки"}, src)
	if f[model.FieldNeed] == "" {
		t.Error("сжатая честная формулировка удалена")
	}
}

func TestPromptListsAllFields(t *testing.T) {
	for k, label := range model.FieldLabels {
		if !strings.Contains(PromptQuestions, label) || !strings.Contains(PromptQuestions, string(k)) {
			t.Errorf("PromptQuestions не содержит поле %s — %s", k, label)
		}
	}
}

func responsesBody(t *testing.T, payload any) []byte {
	t.Helper()
	inner, _ := json.Marshal(payload)
	b, _ := json.Marshal(map[string]any{
		"output": []any{map[string]any{
			"type":    "message",
			"content": []any{map[string]any{"type": "output_text", "text": string(inner)}},
		}},
	})
	return b
}

func TestFallbackOn500(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newClient("test-key", "", srv.URL)
	r, err := c.Questions(context.Background(), weakDraft, "Логистика")
	if err != nil {
		t.Fatal(err)
	}
	checkQuestions(t, r)
	if c.Mode() != model.AIModeMock {
		t.Errorf("mode = %s, ожидался mock", c.Mode())
	}
	info := c.Info()
	if info.LastError == nil || !strings.Contains(*info.LastError, "500") {
		t.Errorf("last_error = %v", info.LastError)
	}
	if hits.Load() != 2 {
		t.Errorf("ожидался один повтор (2 запроса), было %d", hits.Load())
	}
}

func TestOpenAIValidResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer test-key" ||
			r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var req responsesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Model != defaultModel ||
			req.Text.Format.Type != "json_schema" || !req.Text.Format.Strict || len(req.Input) != 2 {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if req.Text.Format.Name == "task_card" {
			w.Write(responsesBody(t, map[string]any{"fields": map[string]string{
				"title": "Учёт заявок", "data": "Выгрузка заявок из Excel", "constraints": "Бюджет 5 млн тенге",
			}}))
			return
		}
		w.Write(responsesBody(t, map[string]any{
			"questions": []map[string]string{
				{"text": "Какой контекст?", "field_key": "context"},
				{"text": "Какие данные?", "field_key": "data"},
				{"text": "Как измерить успех?", "field_key": "success_criteria"},
			},
			"missing_fields": []string{"context", "data", "success_criteria"},
		}))
	}))
	defer srv.Close()

	c := newClient("test-key", "", srv.URL)
	r, err := c.Questions(context.Background(), weakDraft, "Логистика")
	if err != nil {
		t.Fatal(err)
	}
	checkQuestions(t, r)
	if c.Mode() != model.AIModeOpenAI {
		t.Errorf("mode = %s, ожидался openai; last_error=%v", c.Mode(), c.Info().LastError)
	}
	if r.Questions[0].Text != "Какой контекст?" {
		t.Errorf("вопросы не из ответа модели: %+v", r.Questions)
	}

	// Card через openai: guard выкидывает выдуманный бюджет
	card, err := c.Card(context.Background(), weakDraft, "Логистика",
		[]model.Question{{ID: 1, FieldKey: model.FieldData, Answer: "Выгрузка заявок из Excel"}})
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode() != model.AIModeOpenAI {
		t.Errorf("mode = %s", c.Mode())
	}
	if card.Fields[model.FieldConstraints] != "" || card.Fields[model.FieldData] == "" {
		t.Errorf("guard в openai-режиме: %+v", card.Fields)
	}
}

func TestOpenAIPadsTooFewQuestions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(responsesBody(t, map[string]any{
			"questions":      []map[string]string{{"text": "Какие данные?", "field_key": "data"}},
			"missing_fields": []string{"data", "users"},
		}))
	}))
	defer srv.Close()
	c := newClient("k", "", srv.URL)
	r, _ := c.Questions(context.Background(), weakDraft, "")
	checkQuestions(t, r)
	if r.Questions[1].FieldKey != model.FieldUsers {
		t.Errorf("дополнение не по missing_fields: %+v", r.Questions)
	}
	if c.Mode() != model.AIModeOpenAI {
		t.Errorf("mode = %s", c.Mode())
	}
}

func TestOpenAIBadStructureFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"unexpected": true}`))
	}))
	defer srv.Close()
	c := newClient("k", "", srv.URL)
	r, err := c.Questions(context.Background(), weakDraft, "")
	if err != nil {
		t.Fatal(err)
	}
	checkQuestions(t, r)
	if c.Mode() != model.AIModeMock || c.Info().LastError == nil {
		t.Errorf("mode = %s, last_error = %v", c.Mode(), c.Info().LastError)
	}
}

// need в mock берётся из черновика от маркера до конца предложения; маркер сохраняется.
func TestMockCardNeedFromDraft(t *testing.T) {
	r, err := New("", "").Card(context.Background(), weakDraft, "Логистика", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Fields[model.FieldNeed]; got != "Нужно приложение для учёта заявок" {
		t.Errorf("need = %q", got)
	}
	cases := map[string]string{
		"Сейчас всё в таблицах, поэтому хотим бота для склада. Срок месяц": "Хотим бота для склада",
		"Требуется: отчёт по продажам!":                                    "Требуется: отчёт по продажам",
		"Заявки идут в почту.\nНадо разбирать их быстрее, —":               "Надо разбирать их быстрее",
		"Просто описание без маркера":                                      "",
		"Нужно.": "",
	}
	for in, want := range cases {
		if got := needFragment(in); got != want {
			t.Errorf("needFragment(%q) = %q, want %q", in, got, want)
		}
	}
	// ответ на вопрос про need важнее черновика
	r, _ = New("", "").Card(context.Background(), weakDraft, "", []model.Question{
		{ID: 1, FieldKey: model.FieldNeed, Answer: "Учёт заявок в одном месте"}})
	if r.Fields[model.FieldNeed] != "Учёт заявок в одном месте" {
		t.Errorf("need = %q", r.Fields[model.FieldNeed])
	}
}

func TestFirstWordsNoTrailingServiceWord(t *testing.T) {
	cases := map[string]string{
		"Нашим менеджерам нужно быстрее обрабатывать входящие заявки клиентов, сейчас всё в таблицах": "Нашим менеджерам нужно быстрее обрабатывать входящие заявки клиентов",
		"Хотим бота для склада и":                                                   "Хотим бота для склада",
		"Учёт заявок — сейчас всё в таблицах и почте":                               "Учёт заявок — сейчас всё в таблицах",
		"Нашим менеджерам нужно быстрее обрабатывать заявки, сейчас всё в таблицах": "Нашим менеджерам нужно быстрее обрабатывать заявки, сейчас",
		"Отчёт по продажам, без лишнего":                                            "Отчёт по продажам, без лишнего",
		"Отчёт по продажам,":                                                        "Отчёт по продажам",
		"":                                                                          "",
	}
	for in, want := range cases {
		got := firstWords(in, 8)
		if got != want {
			t.Errorf("firstWords(%q) = %q, want %q", in, got, want)
		}
		ws := strings.Fields(got)
		if len(ws) > 1 && titleStopWords[strings.ToLower(ws[len(ws)-1])] {
			t.Errorf("firstWords(%q) заканчивается служебным словом: %q", in, got)
		}
	}
}

// Число засчитывается, только если в источнике оно стоит рядом с тем же словом.
func TestGuardNumberNeedsSameNeighbour(t *testing.T) {
	src := "Сейчас 30 операторов обрабатывают 500 заявок в день, срок 2 недели"
	f := Guard(model.Fields{
		model.FieldNeed:            "Снизить число операторов до 2, обрабатывать 500 заявок",
		model.FieldContext:         "500 заявок в день",
		model.FieldConstraints:     "Срок 2 недели",
		model.FieldSuccessCriteria: "Сейчас 30 операторов", // число в середине
		model.FieldUsers:           "операторов 30",        // число в конце: сосед слева не совпадает
	}, src)
	if f[model.FieldNeed] != "" {
		t.Errorf("перенесённое число не удалено: %q", f[model.FieldNeed])
	}
	if f[model.FieldContext] == "" || f[model.FieldConstraints] == "" || f[model.FieldSuccessCriteria] == "" {
		t.Errorf("честные поля с числами удалены: %+v", f)
	}
	if f[model.FieldUsers] != "" {
		t.Errorf("число с чужим соседом сохранено: %q", f[model.FieldUsers])
	}
}

// title проверяется Guard'ом; выдуманный заменяется началом черновика.
func TestGuardChecksTitle(t *testing.T) {
	const draft = "хотим бота для склада"
	title := "Сбербанк: ИИ-платформа на 10 млн клиентов"
	if f := Guard(model.Fields{model.FieldTitle: title}, draft); f[model.FieldTitle] != "" {
		t.Errorf("выдуманный title сохранён: %q", f[model.FieldTitle])
	}
	r := finishCard(model.Fields{model.FieldTitle: title}, draft, "", nil)
	if r.Fields[model.FieldTitle] != firstWords(draft, 8) || r.Fields[model.FieldTitle] != "хотим бота для склада" {
		t.Errorf("title = %q", r.Fields[model.FieldTitle])
	}
}

// Стем 4/5 букв без окончания: честный пересказ с другой формой слова не стирается.
func TestGuardKeepsOtherWordForms(t *testing.T) {
	src := "Сейчас много заявок, их вручную ведут в таблице, и часть теряются. Хотим сократить время ответа с 2 дней до 4 часов"
	f := Guard(model.Fields{
		model.FieldContext:         "Заявки регистрируются вручную в таблицах и теряются",
		model.FieldSuccessCriteria: "Сократить время ответа с 2 дня до 4 часа",
		model.FieldNeed:            "Сократить время ответа с 2 дня до 4 недель", // число с чужим словом
	}, src)
	if f[model.FieldContext] == "" || f[model.FieldSuccessCriteria] == "" {
		t.Errorf("честный пересказ стёрт: %+v", f)
	}
	if f[model.FieldNeed] != "" {
		t.Errorf("число с чужим словом сохранено: %q", f[model.FieldNeed])
	}
}

// Слово с заглавной буквы не в начале предложения, которого нет в источнике, очищает поле.
func TestGuardDropsInventedProperName(t *testing.T) {
	src := "Менеджеры вручную переносят заявки клиентов из таблицы в отчёт"
	f := Guard(model.Fields{
		model.FieldNeed:    "Менеджеры переносят заявки клиентов из Kaspi вручную", // 5 из 6 слов совпали
		model.FieldContext: "Заявки клиентов менеджеры переносят вручную. Отчёт из таблицы",
	}, src)
	if f[model.FieldNeed] != "" {
		t.Errorf("выдуманный бренд сохранён: %q", f[model.FieldNeed])
	}
	if f[model.FieldContext] == "" {
		t.Error("заглавная в начале предложения принята за имя")
	}
}

// Отрицание в поле при утверждении в источнике (и наоборот) очищает поле.
func TestGuardDropsFlippedNegation(t *testing.T) {
	pos := "Данные есть, выгрузки CRM передадим после NDA"
	neg := "Данных нет, выгрузки CRM не передадим"
	if f := Guard(model.Fields{model.FieldData: neg}, pos); f[model.FieldData] != "" {
		t.Errorf("отрицание при утверждении в источнике сохранено: %q", f[model.FieldData])
	}
	if f := Guard(model.Fields{model.FieldData: "Данные есть, выгрузки CRM передадим"}, neg); f[model.FieldData] != "" {
		t.Errorf("утверждение при отрицании в источнике сохранено: %q", f[model.FieldData])
	}
	src := "персональных данных нет, работаем с логами"
	f := Guard(model.Fields{model.FieldConstraints: "персональных данных нет", model.FieldData: "работаем с логами"}, src)
	if f[model.FieldConstraints] == "" || f[model.FieldData] == "" {
		t.Errorf("честное отрицание стёрто: %+v", f)
	}
}
