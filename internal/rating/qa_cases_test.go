package rating

import (
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// Кейсы из research/rating-qa-cases.md (независимый QA): B и D — баги, C — задокументированное правило.
func TestQAFixtureCases(t *testing.T) {
	cases := []struct {
		name string
		key  model.FieldKey
		val  string
		want int
	}{
		{"B: доступны + позже = нет плана", model.FieldData, "Данные доступны, передадим позже.", 10},
		{"D: приёмщики — роль", model.FieldUsers, "Приёмщики заказов координируют очередь обращений, проверяют полноту описания и передают заказ в работу.", 10},
		{"C: число с единицей — измеримо (правило)", model.FieldSuccessCriteria, "Улучшить качество на 50%", 15},
		{"B': доступ с конкретикой", model.FieldData, "Выгрузка из CRM доступна, дадим доступ в первую неделю.", 20},
	}
	for _, c := range cases {
		r := Compute(model.Fields{c.key: c.val}, true)
		got := -1
		for _, b := range r.Breakdown {
			if (c.key == model.FieldData && b.Key == "data") || (c.key == model.FieldUsers && b.Key == "users") || (c.key == model.FieldSuccessCriteria && b.Key == "success_criteria") {
				got = b.Earned
			}
		}
		if got != c.want {
			t.Errorf("%s: earned=%d, want %d", c.name, got, c.want)
		}
	}
}
