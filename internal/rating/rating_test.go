package rating

import (
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

func fullCard() model.Fields {
	return model.Fields{
		model.FieldTitle:             "Автоматизация обработки заявок",
		model.FieldContext:           "Сейчас заявки клиентов приходят на почту и вручную переносятся в таблицу, это занимает много времени.",
		model.FieldNeed:              "Нужно автоматически классифицировать заявки и распределять их по ответственным менеджерам.",
		model.FieldUsers:             "Менеджеры отдела продаж и руководители филиалов компании, около 40 сотрудников.",
		model.FieldData:              "Есть выгрузка из CRM за 2 года в Excel, около 50 000 заявок с примерами категорий, передадим в первую неделю.",
		model.FieldConstraints:       "Срок — 2 месяца, данные нельзя выносить за контур, стек только Python и PostgreSQL.",
		model.FieldExpectedResult:    "Прототип веб-сервиса, который классифицирует входящие заявки и назначает ответственного.",
		model.FieldSuccessCriteria:   "Точность классификации не менее 85%, время обработки заявки снижение на 30% за 3 месяца.",
		model.FieldContact:           "Иван Петров, ivan.petrov@example.kz",
		model.FieldInteractionFormat: "Еженедельный созвон",
	}
}

func sumEarned(r model.Rating) int {
	s := 0
	for _, b := range r.Breakdown {
		s += b.Earned
	}
	return s
}

func checkInvariants(t *testing.T, r model.Rating) {
	t.Helper()
	if len(r.Breakdown) != len(Weights) {
		t.Fatalf("breakdown len = %d, want %d", len(r.Breakdown), len(Weights))
	}
	for i, b := range r.Breakdown {
		w := Weights[i]
		if b.Key != w.Key || b.Weight != w.Weight || b.Label != w.Label {
			t.Errorf("breakdown[%d] = %+v, want key/label/weight of %+v", i, b, w)
		}
		if b.Earned != 0 && b.Earned != w.Weight/2 && b.Earned != w.Weight {
			t.Errorf("%s earned %d not in {0,%d,%d}", b.Key, b.Earned, w.Weight/2, w.Weight)
		}
		if b.Reason == "" {
			t.Errorf("%s: empty reason", b.Key)
		}
	}
	if s := sumEarned(r); s != r.Score {
		t.Errorf("sum(breakdown) = %d, score = %d", s, r.Score)
	}
	for i := 1; i < len(r.Missing); i++ {
		if r.Missing[i-1].Gain < r.Missing[i].Gain {
			t.Errorf("missing not sorted by gain: %+v", r.Missing)
		}
	}
	if r.Level != LevelFor(r.Score) || r.LevelLabel != model.LevelLabels[r.Level] {
		t.Errorf("level mismatch: %s/%s for %d", r.Level, r.LevelLabel, r.Score)
	}
}

func TestWeightsSum(t *testing.T) {
	s := 0
	for _, w := range Weights {
		s += w.Weight
	}
	if s != 100 {
		t.Fatalf("weights sum = %d", s)
	}
}

func TestEmpty(t *testing.T) {
	for _, f := range []model.Fields{nil, {}, {model.FieldContext: "   "}} {
		r := Compute(f, false)
		checkInvariants(t, r)
		if r.Score != 0 || r.Level != model.LevelDraft || r.LevelLabel != "черновик" {
			t.Fatalf("empty: got %d %s %s", r.Score, r.Level, r.LevelLabel)
		}
		if len(r.Missing) != 7 || r.Missing[0].Key != "context_need" || r.Missing[1].Key != "data" {
			t.Fatalf("missing = %+v", r.Missing)
		}
		for _, b := range r.Breakdown {
			if b.Reason != "Не заполнено" {
				t.Errorf("%s reason = %q", b.Key, b.Reason)
			}
		}
	}
}

func TestFull(t *testing.T) {
	for _, confirmed := range []bool{false, true} {
		r := Compute(fullCard(), confirmed)
		checkInvariants(t, r)
		if r.Score != 100 || r.Level != model.LevelPriority {
			t.Fatalf("full: score %d level %s; breakdown %+v", r.Score, r.Level, r.Breakdown)
		}
		if len(r.Missing) != 0 {
			t.Fatalf("missing = %+v", r.Missing)
		}
	}
}

func TestPartial(t *testing.T) {
	f := model.Fields{
		model.FieldContext:         "Сейчас заявки клиентов приходят на почту и обрабатываются вручную.",            // полно
		model.FieldData:            "Есть выгрузка из CRM за 2 года в Excel, около 5000 строк, передадим после NDA", // полно
		model.FieldExpectedResult:  "Прототип",                                                                      // коротко → 7
		model.FieldSuccessCriteria: "Чтобы было удобно и быстро работать сотрудникам",                               // без меры → 7
		model.FieldUsers:           "Менеджеры отдела продаж и руководители филиалов компании",                      // полно
		model.FieldContact:         "ivan@corp.kz",                                                                  // полно, формат пуст → 5
	}
	r := Compute(f, false)
	checkInvariants(t, r)
	want := map[string]int{"context_need": 10, "data": 20, "expected_result": 7, "success_criteria": 7,
		"constraints": 0, "users": 10, "business_link": 5}
	for _, b := range r.Breakdown {
		if b.Earned != want[b.Key] {
			t.Errorf("%s earned %d, want %d (%s)", b.Key, b.Earned, want[b.Key], b.Reason)
		}
	}
	if r.Score != 59 || r.Level != model.LevelWorking {
		t.Fatalf("score %d level %s", r.Score, r.Level)
	}
	sc := r.Breakdown[3]
	if !strings.Contains(sc.Reason, "нет измеримого признака") {
		t.Errorf("success_criteria reason = %q", sc.Reason)
	}
	// gain: constraints 10, context_need 10, expected 8, success 8, business 5
	gotOrder := []string{}
	for _, m := range r.Missing {
		gotOrder = append(gotOrder, m.Key)
		if m.Hint == "" {
			t.Errorf("%s: empty hint", m.Key)
		}
	}
	wantOrder := "context_need,constraints,expected_result,success_criteria,business_link"
	if strings.Join(gotOrder, ",") != wantOrder {
		t.Fatalf("missing order = %v, want %s", gotOrder, wantOrder)
	}
}

func TestFieldSigns(t *testing.T) {
	cases := []struct {
		k    model.FieldKey
		text string
		full bool
	}{
		{model.FieldContact, "+7 701 123 45 67", true},
		{model.FieldContact, "8 (701) 123-45-67", true},
		{model.FieldContact, "t.me/ivan_corp", true},
		{model.FieldContact, "Иван, отдел продаж", false},
		{model.FieldInteractionFormat, "созвон раз в две недели", true},
		{model.FieldInteractionFormat, "раз в две недели", false},
		{model.FieldInteractionFormat, "как получится", false},
		{model.FieldConstraints, "Всё нужно сделать до 1 декабря, иначе проект теряет смысл.", true},
		{model.FieldConstraints, "Никаких особых пожеланий к реализации решения у нас нет вообще.", false},
		{model.FieldExpectedResult, "Хотим получить отчёт с анализом продаж по регионам", true},
		{model.FieldSuccessCriteria, "Проект поможет простой и понятной работе всех менеджеров", false},
	}
	for _, c := range cases {
		got := evalField(c.k, c.text).st == stFull
		if got != c.full {
			t.Errorf("%s %q: full=%v, want %v", c.k, c.text, got, c.full)
		}
	}
}

func TestLevels(t *testing.T) {
	cases := map[int]model.Level{0: model.LevelDraft, 39: model.LevelDraft, 40: model.LevelWorking, 69: model.LevelWorking,
		70: model.LevelReady, 89: model.LevelReady, 90: model.LevelPriority, 100: model.LevelPriority}
	for s, want := range cases {
		if got := LevelFor(s); got != want {
			t.Errorf("LevelFor(%d) = %s, want %s", s, got, want)
		}
	}
}

func TestFormulaDescription(t *testing.T) {
	d := FormulaDescription()
	for _, s := range []string{"Контекст и потребность — 20", "Связь с бизнесом — 10", "90–100 — приоритетная"} {
		if !strings.Contains(d, s) {
			t.Errorf("description lacks %q", s)
		}
	}
}

func allFields(v string) model.Fields {
	f := model.Fields{}
	for _, k := range model.FieldKeys {
		f[k] = v
	}
	return f
}

func earnedOf(r model.Rating, key string) int {
	for _, b := range r.Breakdown {
		if b.Key == key {
			return b.Earned
		}
	}
	return -1
}

func TestStubs(t *testing.T) {
	stubs := []string{"x", "XXX", "х", "-", "—", ".", "?", "...", "123", "TODO", "tbd", "N/A", "нет", "Не знаю.",
		"нет данных", "Пока нет", "позже", "заполню", "  x  "}
	for _, s := range stubs {
		r := Compute(allFields(s), false)
		checkInvariants(t, r)
		if r.Score != 0 || r.Level != model.LevelDraft {
			t.Errorf("all fields %q: score %d level %s", s, r.Score, r.Level)
		}
		for _, b := range r.Breakdown {
			if b.Reason != "Не заполнено (заглушка)" {
				t.Errorf("%q %s reason = %q", s, b.Key, b.Reason)
			}
		}
	}
	// Содержательная фраза с «нет» — не заглушка.
	if st := evalField(model.FieldConstraints, "персональных данных нет, работаем с обезличенными логами").st; st == stStub || st == stEmpty {
		t.Errorf("содержательная фраза с «нет» принята за заглушку")
	}
}

func TestCounterexamples(t *testing.T) {
	cases := []struct {
		name string
		f    model.Fields
		key  string
		want int
	}{
		{"contact @ + чат", model.Fields{model.FieldContact: "@", model.FieldInteractionFormat: "чат"}, "business_link", 5},
		{"contact tg + format x", model.Fields{model.FieldContact: "tg", model.FieldInteractionFormat: "x"}, "business_link", 0},
		{"контакт без формы", model.Fields{model.FieldContact: "Иван", model.FieldInteractionFormat: "Созвон раз в неделю"}, "business_link", 5},
		{"telegram + ритм", model.Fields{model.FieldContact: "@ivan_corp", model.FieldInteractionFormat: "Чат в Telegram, отвечаем в течение дня"}, "business_link", 10},
		{"версия не число", model.Fields{model.FieldSuccessCriteria: "Проверить обработку заявок в новой версии 2"}, "success_criteria", 7},
		{"число с метрикой", model.Fields{model.FieldSuccessCriteria: "снизить время обработки заявки на 30%"}, "success_criteria", 15},
		{"рост без числа", model.Fields{model.FieldSuccessCriteria: "рост продаж и снижение нагрузки на операторов"}, "success_criteria", 7},
		{"сценарий приёмки", model.Fields{model.FieldSuccessCriteria: "Принимаем, если на 20 реальных заявках бот верно определит категорию"}, "success_criteria", 15},
		{"данных нет — отказ", model.Fields{model.FieldData: "Данных нет, выгрузка недоступна"}, "data", 0},
		{"материал недоступен без плана", model.Fields{model.FieldData: "Выгрузка из CRM недоступна"}, "data", 10},
		{"данных нет с планом", model.Fields{model.FieldData: "Данных пока нет, соберём образцы заявок в течение недели"}, "data", 20},
		{"материал + передача", model.Fields{model.FieldData: "Выгрузка из CRM за 2 года, передадим в первую неделю"}, "data", 20},
		{"материал без передачи", model.Fields{model.FieldData: "Выгрузка из CRM за 2 года"}, "data", 10},
		{"короткие пользователи", model.Fields{model.FieldUsers: "10 менеджеров сервисного центра"}, "users", 10},
		{"пользователи без роли", model.Fields{model.FieldUsers: "Много людей по всему Казахстану каждый день"}, "users", 5},
		{"короткий артефакт с функцией", model.Fields{model.FieldExpectedResult: "CSV-отчёт по продажам с фильтром по датам"}, "expected_result", 15},
		{"артефакт без функции", model.Fields{model.FieldExpectedResult: "Прототип"}, "expected_result", 7},
	}
	for _, c := range cases {
		r := Compute(c.f, false)
		checkInvariants(t, r)
		if got := earnedOf(r, c.key); got != c.want {
			t.Errorf("%s: %s = %d, want %d (%+v)", c.name, c.key, got, c.want, r.Breakdown)
		}
	}
}

func TestNoLengthHintsAndMeaningfulReasons(t *testing.T) {
	r := Compute(model.Fields{model.FieldContext: "Заявки обрабатываем вручную", model.FieldNeed: "Автоматизировать приём заявок"}, false)
	checkInvariants(t, r)
	b := r.Breakdown[0]
	if b.Earned != 10 || !strings.Contains(b.Reason, "опишите, что происходит сейчас и что нужно изменить") {
		t.Errorf("context_need = %d %q", b.Earned, b.Reason)
	}
	for _, f := range []model.Fields{allFields("коротко"), {}} {
		r := Compute(f, false)
		for _, b := range r.Breakdown {
			if strings.Contains(b.Reason, "символ") || strings.Contains(b.Reason, "слов") {
				t.Errorf("%s reason about length: %q", b.Key, b.Reason)
			}
		}
		for _, m := range r.Missing {
			if strings.Contains(m.Hint, "символ") || m.Hint == "" {
				t.Errorf("%s hint = %q", m.Key, m.Hint)
			}
		}
	}
}

func TestRepeatAndSpacesDoNotHelp(t *testing.T) {
	phrases := model.Fields{
		model.FieldContext:           "Заявки обрабатываем вручную",
		model.FieldNeed:              "Нужно быстрее",
		model.FieldUsers:             "Люди из офиса",
		model.FieldData:              "Есть кое-что",
		model.FieldConstraints:       "Особых нет",
		model.FieldExpectedResult:    "Что-то полезное",
		model.FieldSuccessCriteria:   "Чтобы было лучше",
		model.FieldContact:           "Иван",
		model.FieldInteractionFormat: "Как удобно",
	}
	repeated := model.Fields{}
	for k, v := range phrases {
		repeated[k] = strings.Repeat(v+"   \n\t ", 5)
	}
	one, many := Compute(phrases, false), Compute(repeated, false)
	checkInvariants(t, one)
	checkInvariants(t, many)
	if many.Score > one.Score {
		t.Fatalf("повтор улучшил оценку: %d > %d", many.Score, one.Score)
	}
	for i := range one.Breakdown {
		if many.Breakdown[i].Earned > one.Breakdown[i].Earned {
			t.Errorf("%s: повтор %d > одна фраза %d", one.Breakdown[i].Key, many.Breakdown[i].Earned, one.Breakdown[i].Earned)
		}
	}
}

// Мусор без признаков показателя и короче 3 значимых слов — 0 по каждому показателю.
func TestJunkGivesZero(t *testing.T) {
	for _, v := range []string{"asdf", "сделать хорошо", "Иван", "--- ---", "ok ok ok"} {
		r := Compute(allFields(v), false)
		checkInvariants(t, r)
		for _, b := range r.Breakdown {
			if b.Earned != 0 {
				t.Errorf("%q: %s = %d (%s)", v, b.Key, b.Earned, b.Reason)
			}
		}
	}
	r := Compute(allFields("сделать хорошо"), false)
	if got := r.Breakdown[1].Reason; got != "Не заполнено: нет сведений по показателю" {
		t.Errorf("reason = %q", got)
	}
	// Короткий текст с признаком — не мусор.
	cases := []struct {
		k    model.FieldKey
		text string
		want state
	}{
		{model.FieldUsers, "10 менеджеров сервисного центра", stFull},
		{model.FieldExpectedResult, "Прототип", stPartial},
		{model.FieldInteractionFormat, "Еженедельный созвон", stFull},
		{model.FieldContext, "Заявки вручную", stJunk},
		{model.FieldNeed, "Автоматизировать приём заявок", stPartial},
	}
	for _, c := range cases {
		if got := evalField(c.k, c.text).st; got != c.want {
			t.Errorf("%s %q: state %d, want %d", c.k, c.text, got, c.want)
		}
	}
}

// Отказ без плана или альтернативы — 0 с причиной «Сведения отсутствуют (отказ)».
func TestRefusalGivesZero(t *testing.T) {
	cases := []struct {
		k    model.FieldKey
		key  string
		text string
	}{
		{model.FieldData, "data", "Никаких файлов не передадим, данных у нас не будет никогда"},
		{model.FieldSuccessCriteria, "success_criteria", "Критериев нет, тест не планируем, 0 процентов"},
		{model.FieldUsers, "users", "Пользователей нет, клиентов тоже"},
		{model.FieldInteractionFormat, "business_link", "Встреч не будет, чат не ведём каждый день"},
	}
	for _, c := range cases {
		r := Compute(model.Fields{c.k: c.text}, false)
		checkInvariants(t, r)
		for _, b := range r.Breakdown {
			if b.Key == c.key && (b.Earned != 0 || b.Reason != "Сведения отсутствуют (отказ)") {
				t.Errorf("%q: %s = %d %q", c.text, b.Key, b.Earned, b.Reason)
			}
		}
	}
	for _, text := range []string{"персональных данных нет, работаем с обезличенными логами",
		"Данных пока нет, соберём образцы заявок в течение недели"} {
		if st := evalField(model.FieldConstraints, text).st; st == stRefusal {
			t.Errorf("%q принят за отказ", text)
		}
	}
}

// Слова-заглушки внутри текста: без них остаётся меньше 3 значимых слов — 0.
func TestStubWordsInsideText(t *testing.T) {
	for _, v := range []string{"xxx 1 раз", "x клиент", "tbd до", "todo сервис", "тест тест ок да",
		"не знаю не знаю пока нет информации позже заполню обязательно честно"} {
		r := Compute(allFields(v), false)
		checkInvariants(t, r)
		if r.Score != 0 {
			t.Errorf("%q: score %d, %+v", v, r.Score, r.Breakdown)
		}
	}
}

func TestPhoneNeedsDistinctDigits(t *testing.T) {
	for _, v := range []string{"0000000000", "+7 000 000 00 00"} {
		r := Compute(model.Fields{model.FieldContact: v, model.FieldInteractionFormat: "x"}, false)
		if got := earnedOf(r, "business_link"); got != 0 {
			t.Errorf("contact %q: business_link = %d", v, got)
		}
	}
	if evalField(model.FieldContact, "+7 701 123 45 67").reason != "указан телефон" {
		t.Error("настоящий телефон не принят")
	}
}

// Причина полного балла называет найденный признак.
func TestFullReasonNamesSign(t *testing.T) {
	want := map[string][]string{
		"context_need":     {"описана текущая ситуация", "описана потребность"},
		"data":             {"назван материал (выгрузка)", "способ или срок передачи"},
		"expected_result":  {"назван артефакт (прототип)"},
		"success_criteria": {"есть число с единицей (85%)"},
		"constraints":      {"указано ограничение (срок)"},
		"users":            {"названа роль (менеджеры)"},
		"business_link":    {"указан email", "указаны канал и ритм общения"},
	}
	for _, b := range Compute(fullCard(), true).Breakdown {
		if !strings.HasPrefix(b.Reason, "Заполнено полностью: ") {
			t.Errorf("%s reason = %q", b.Key, b.Reason)
		}
		for _, w := range want[b.Key] {
			if !strings.Contains(b.Reason, w) {
				t.Errorf("%s reason %q не содержит %q", b.Key, b.Reason, w)
			}
		}
	}
}

// Кейсы QA.
func TestQACases(t *testing.T) {
	cases := []struct {
		name   string
		f      model.Fields
		key    string
		want   int
		reason string
	}{
		{"роль сотрудники", model.Fields{model.FieldUsers: "Сотрудники приёма заказов координируют очередь, проверяют полноту описания и передают работу"}, "users", 10, "названа роль (сотрудники)"},
		{"приёмка без числа", model.Fields{model.FieldSuccessCriteria: "Начальник смены принимает результат после проверки пилота, когда оператор сам находит заказ и меняет статус"}, "success_criteria", 15, "описан способ проверки"},
		{"передадим позже", model.Fields{model.FieldData: "Данные есть, передадим позже"}, "data", 10, "не указано, как и когда"},
		{"передадим файлом через неделю", model.Fields{model.FieldData: "Данные есть, передадим файлом через неделю"}, "data", 20, ""},
		{"абстрактная метрика", model.Fields{model.FieldSuccessCriteria: "Улучшить качество на 50%"}, "success_criteria", 15, "измеримо, но не сказано, как измеряется качество"},
	}
	for _, c := range cases {
		r := Compute(c.f, false)
		checkInvariants(t, r)
		for _, b := range r.Breakdown {
			if b.Key != c.key {
				continue
			}
			if b.Earned != c.want || !strings.Contains(b.Reason, c.reason) {
				t.Errorf("%s: %s = %d %q, want %d %q", c.name, b.Key, b.Earned, b.Reason, c.want, c.reason)
			}
		}
	}
	if r := Compute(fullCard(), true); strings.Contains(r.Breakdown[3].Reason, "измеримо, но") {
		t.Errorf("ложное предупреждение: %q", r.Breakdown[3].Reason)
	}
}

// Отрицание рядом с ключевым словом обнуляет признак: остаток без «не/нет/ни/никаких/никто/без + до 3 слов».
func TestNegationNearSignGivesZero(t *testing.T) {
	cases := []struct {
		k    model.FieldKey
		key  string
		text string
	}{
		{model.FieldData, "data", "Никаких данных, выгрузок и файлов вы не получите, доступ к CRM закрыт"},
		{model.FieldExpectedResult, "expected_result", "Не нужен ни прототип ни сервис ни отчёт"},
		{model.FieldUsers, "users", "Никто, ни сотрудники ни клиенты пользоваться не будут"},
		{model.FieldInteractionFormat, "business_link", "Чат раз в год, отвечать не будем"},
	}
	for _, c := range cases {
		r := Compute(model.Fields{c.k: c.text}, false)
		checkInvariants(t, r)
		if b := r.Breakdown[indexOf(c.key)]; b.Earned != 0 || b.Reason != "Сведения отсутствуют (отказ)" {
			t.Errorf("%q: %s = %d %q", c.text, c.key, b.Earned, b.Reason)
		}
	}
	// Отрицание-уточнение при сохранившемся признаке не обнуляет.
	keep := []struct {
		k    model.FieldKey
		text string
	}{
		{model.FieldUsers, "Сотрудники склада без доступа к ПК"},
		{model.FieldData, "Выгрузим журнал заявок без персональных данных в первую неделю"},
		{model.FieldInteractionFormat, "Созвон раз в неделю, в выходные отвечать не будем"},
		{model.FieldData, "Выгрузки CRM нет, но есть таблицы Excel, передадим после NDA"},
	}
	for _, c := range keep {
		if st := evalField(c.k, c.text).st; st != stFull {
			t.Errorf("%s %q: state %d, want full", c.k, c.text, st)
		}
	}
}

func indexOf(key string) int {
	for i, w := range Weights {
		if w.Key == key {
			return i
		}
	}
	return -1
}

// Фразы-заглушки внутри текста: без них остаётся меньше 3 значимых слов — 0.
func TestStubPhrasesInsideText(t *testing.T) {
	cases := []struct {
		k    model.FieldKey
		key  string
		text string
	}{
		{model.FieldContext, "context_need", "Здесь будет описание контекста, заполним позднее обязательно"},
		{model.FieldUsers, "users", "пользователи (уточним)"},
		{model.FieldExpectedResult, "expected_result", "какой-нибудь сервис для чего-нибудь"},
		{model.FieldSuccessCriteria, "success_criteria", "примем если понравится"},
		{model.FieldData, "data", "TBD: выгрузка потом"},
	}
	for _, c := range cases {
		r := Compute(model.Fields{c.k: c.text}, false)
		checkInvariants(t, r)
		if got := earnedOf(r, c.key); got != 0 {
			t.Errorf("%q: %s = %d", c.text, c.key, got)
		}
	}
	// «не позднее» — срок, а не заглушка; содержательный текст с «уточним» остаётся.
	if st := evalField(model.FieldExpectedResult, "Отчёт по продажам с фильтром по датам, детали уточним на созвоне").st; st != stFull {
		t.Errorf("содержательный результат с «уточним»: state %d", st)
	}
	if st := evalField(model.FieldInteractionFormat, "Созвон по вторникам, отвечаем не позднее следующего дня").st; st != stFull {
		t.Errorf("«не позднее» принято за заглушку: state %d", st)
	}
}

// Бессмыслица в контексте/потребности — 0 с отдельной причиной; обычный абзац — как раньше.
func TestGibberishContextNeed(t *testing.T) {
	for _, f := range []model.Fields{
		{model.FieldContext: "абвгд еёжзи йклмн опрст уфхцч шщъыь эюя фыва олдж"},
		{model.FieldNeed: "qwerty asdfgh zxcvbn"},
		{model.FieldContext: "абвгд еёжзи йклмн опрст уфхцч шщъыь эюя фыва олдж", model.FieldNeed: "qwerty asdfgh zxcvbn"},
	} {
		r := Compute(f, false)
		checkInvariants(t, r)
		if b := r.Breakdown[0]; b.Earned != 0 || b.Reason != "Не заполнено: текст не похож на описание" {
			t.Errorf("%v: context_need = %d %q", f, b.Earned, b.Reason)
		}
	}
	card := fullCard()
	for _, k := range []model.FieldKey{model.FieldContext, model.FieldNeed} {
		if st := evalField(k, card[k]).st; st != stFull {
			t.Errorf("обычный абзац %s: state %d", k, st)
		}
	}
	if st := evalField(model.FieldContext, "Заявки в CRM и 1С вручную переносят менеджеры, теряется время").st; st != stFull {
		t.Errorf("абзац с аббревиатурами: state %d", st)
	}
}

// Простые последовательности цифр — не телефон.
func TestPhoneRejectsSequences(t *testing.T) {
	for _, v := range []string{"1234567890", "0123456789", "1231231231", "+7 123 456 7890", "8 (123) 123-12-31", "9876543210"} {
		if kind := contactKind(v); kind != "" {
			t.Errorf("%q принят как %s", v, kind)
		}
	}
	for _, v := range []string{"+7 701 123 45 67", "8 (701) 123-45-67"} {
		if contactKind(v) != "телефон" {
			t.Errorf("%q не принят", v)
		}
	}
}
