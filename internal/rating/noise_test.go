package rating

import (
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

func TestIsNoise(t *testing.T) {
	for _, s := range []string{"здравствуйте", "Пока не знаю", "пока ничего", "не знаю", "x", "нет данных", ""} {
		if !IsNoise(model.FieldContext, s) {
			t.Errorf("%q должно быть шумом", s)
		}
	}
	for _, s := range []string{"Языковая школа, 120 учеников, заявки теряются", "Excel и WhatsApp", "администратор и два преподавателя"} {
		if IsNoise(model.FieldContext, s) {
			t.Errorf("%q не шум", s)
		}
	}
}
