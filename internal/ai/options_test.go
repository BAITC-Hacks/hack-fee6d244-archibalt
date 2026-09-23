package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// слабый пример языкового центра (research/product-interview-2026-09-23.md)
const centerDraft = "Языковой центр: посещения, оплаты и домашние задания разбросаны между Excel и WhatsApp"

var centerAnswers = []model.Question{
	{ID: 1, FieldKey: model.FieldData, Answer: "Таблицы Excel с посещениями и оплатами за 2 года"},
	{ID: 2, FieldKey: model.FieldUsers, Answer: "Преподаватели и администраторы"},
	{ID: 3, FieldKey: model.FieldSuccessCriteria, Answer: ""},
}

func checkOptions(t *testing.T, opts []model.ResultOption) {
	t.Helper()
	if len(opts) < minOptions || len(opts) > maxOptions {
		t.Fatalf("ожидалось 2–3 варианта, получено %d: %+v", len(opts), opts)
	}
	for i, o := range opts {
		if o.Title == "" || o.Result == "" || o.Check == "" || o.Needs == "" || o.Weeks < minWeeks || o.Weeks > maxWeeks {
			t.Errorf("плохой вариант %d: %+v", i, o)
		}
		if i > 0 && opts[i-1].Weeks > o.Weeks {
			t.Errorf("варианты не от малого к среднему: %+v", opts)
		}
	}
}

func TestMockResultOptions(t *testing.T) {
	c := New("", "")
	r, err := c.ResultOptions(context.Background(), centerDraft, "Образование", centerAnswers)
	if err != nil {
		t.Fatal(err)
	}
	checkOptions(t, r.Options)
	if r.Options[0].Title == r.Options[1].Title || !strings.Contains(r.Options[0].Result, "посещениями") ||
		!strings.Contains(r.Options[1].Title, "преподаватели") {
		t.Errorf("варианты не из ответов пользователя: %+v", r.Options)
	}
	again, _ := c.ResultOptions(context.Background(), centerDraft, "Образование", centerAnswers)
	if again.Options[1] != r.Options[1] {
		t.Error("mock недетерминирован")
	}
	// без ответов — неизвестное названо неизвестным, не выдумано
	empty, _ := c.ResultOptions(context.Background(), "Нужно приложение для учёта заявок", "IT", nil)
	checkOptions(t, empty.Options)
	if !strings.Contains(empty.Options[0].Needs, "неизвестно") {
		t.Errorf("неизвестные данные не помечены: %+v", empty.Options[0])
	}
}

func TestGuardOptionsDropsInvented(t *testing.T) {
	src := sourceText(centerDraft, "Образование", centerAnswers)
	ok := model.ResultOption{Title: "Сводка посещений", Result: "Таблица посещений за 2 года из Excel.", Check: "Администратор находит оплату ученика без WhatsApp.", Needs: "Выгрузка Excel.", Weeks: 3}
	num := ok
	num.Check = "Время сверки сокращается на 50%."
	brand := ok
	brand.Needs = "Доступ к 1С и Telegram-боту."
	got := guardOptions([]model.ResultOption{ok, num, brand}, src)
	if len(got) != 1 || got[0] != ok {
		t.Fatalf("guardOptions: %+v", got)
	}
	// после guard меньше 2 — добивка из mock, не больше 3
	fin := finishOptions([]model.ResultOption{ok, num, brand}, centerDraft, "Образование", centerAnswers)
	checkOptions(t, fin)
	if !hasTitle(fin, ok.Title) {
		t.Errorf("валидный вариант потерян: %+v", fin)
	}
	many := []model.ResultOption{ok, {Title: "Б", Result: "р", Check: "ч", Needs: "н", Weeks: 20}, {Title: "В", Result: "р", Check: "ч", Needs: "н"}, {Title: "Г", Result: "р", Check: "ч", Needs: "н", Weeks: 4}}
	fin = finishOptions(many, centerDraft, "", centerAnswers)
	checkOptions(t, fin)
	if len(fin) != 3 {
		t.Errorf("ожидалось 3 варианта: %+v", fin)
	}
}

func TestOpenAIResultOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(responsesBody(t, map[string]any{"options": []map[string]any{
			{"title": "Сводка посещений группы", "result": "Одна таблица посещений и оплат одной группы из Excel.", "check": "Администратор находит оплату ученика без поиска в WhatsApp.", "needs": "Выгрузка таблиц Excel; кто её передаст — уточнить.", "weeks": 2},
			{"title": "Прототип журнала", "result": "Прототип журнала для преподавателей.", "check": "Преподаватели отмечают посещения в прототипе.", "needs": "Время преподавателей на пробу.", "weeks": 5},
			{"title": "Бот в Telegram", "result": "Бот напоминает об оплате.", "check": "Оплаты приходят на 30% быстрее.", "needs": "Доступ к Telegram.", "weeks": 7},
		}}))
	}))
	defer srv.Close()
	c := newClient("test-key", "", srv.URL)
	r, err := c.ResultOptions(context.Background(), centerDraft, "Образование", centerAnswers)
	if err != nil {
		t.Fatal(err)
	}
	checkOptions(t, r.Options)
	if c.Mode() != model.AIModeOpenAI {
		t.Errorf("mode = %s, last_error=%v", c.Mode(), c.Info().LastError)
	}
	if len(r.Options) != 2 || r.Options[0].Title != "Сводка посещений группы" || hasTitle(r.Options, "Бот в Telegram") {
		t.Errorf("ожидались 2 варианта модели без выдуманного бота: %+v", r.Options)
	}
}

func TestOpenAIResultOptionsEmptyFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(responsesBody(t, map[string]any{"options": []any{}}))
	}))
	defer srv.Close()
	c := newClient("test-key", "", srv.URL)
	r, err := c.ResultOptions(context.Background(), centerDraft, "Образование", centerAnswers)
	if err != nil {
		t.Fatal(err)
	}
	checkOptions(t, r.Options)
	if c.Mode() != model.AIModeMock {
		t.Errorf("mode = %s, ожидался mock", c.Mode())
	}
}
