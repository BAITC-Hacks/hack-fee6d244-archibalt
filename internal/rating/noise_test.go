package rating

import (
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

func TestIsNoise(t *testing.T) {
	for _, s := range []string{"здравствуйте", "Пока не знаю", "пока ничего", "по-разному", "не знаю", "x", "нет данных", ""} {
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

// Расплывчатые ответы — шум только для context/need/users; для данных и ограничений это факт.
func TestIsNoiseVagueByField(t *testing.T) {
	for _, s := range []string{"всё", "Все", "пока ничего", "по-разному"} {
		if !IsNoise(model.FieldUsers, s) || !IsNoise(model.FieldNeed, s) {
			t.Errorf("%q для users/need должно быть шумом", s)
		}
	}
	for _, s := range []string{"пока ничего", "ничего", "пока нет"} {
		if IsNoise(model.FieldData, s) || IsNoise(model.FieldConstraints, s) {
			t.Errorf("%q для data/constraints — факт, не шум", s)
		}
	}
	if !IsNoise(model.FieldData, "не знаю") || !IsNoise(model.FieldData, "здравствуйте") {
		t.Error("«не знаю»/приветствие — шум для любого поля")
	}
}

func TestIsGibberish(t *testing.T) {
	if !IsGibberish("фвфовфыов о") || !IsGibberish("") {
		t.Error("набор букв и пустота — бессмыслица")
	}
	if IsGibberish("Языковой центр, 300 студентов, заявки приходят в WhatsApp и теряются") {
		t.Error("осмысленный черновик принят за бессмыслицу")
	}
}
