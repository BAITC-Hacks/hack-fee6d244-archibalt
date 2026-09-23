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
		model.FieldData:              "Есть выгрузка из CRM за 2 года в Excel, около 50 000 заявок с примерами категорий.",
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
		model.FieldContext:         "Сейчас заявки клиентов приходят на почту и обрабатываются вручную.", // полно
		model.FieldData:            "Есть выгрузка из CRM за 2 года в Excel, около 5000 строк",           // полно
		model.FieldExpectedResult:  "Прототип",                                                           // коротко → 7
		model.FieldSuccessCriteria: "Чтобы было удобно и быстро работать сотрудникам",                    // без меры → 7
		model.FieldUsers:           "Менеджеры отдела продаж и руководители филиалов компании",           // полно
		model.FieldContact:         "ivan@corp.kz",                                                       // полно, формат пуст → 5
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
		{model.FieldInteractionFormat, "раз в две недели", true},
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
