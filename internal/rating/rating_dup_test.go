package rating

import (
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// Одинаковые «Контекст» и «Потребность» дают половину веса индикатора, а не полный.
func TestContextNeedDuplicateHalfWeight(t *testing.T) {
	same := "Языковая школа в Астане, 120 учеников, заявки из WhatsApp теряются каждую неделю"
	dup := Compute(model.Fields{model.FieldContext: same, model.FieldNeed: same}, false)
	diff := Compute(model.Fields{model.FieldContext: same, model.FieldNeed: "Нужен единый список заявок с ответственным и статусом"}, false)
	if dup.Breakdown[0].Earned >= diff.Breakdown[0].Earned || dup.Breakdown[0].Earned != Weights[0].Weight/2 {
		t.Fatalf("dup=%d diff=%d weight=%d", dup.Breakdown[0].Earned, diff.Breakdown[0].Earned, Weights[0].Weight)
	}
}
