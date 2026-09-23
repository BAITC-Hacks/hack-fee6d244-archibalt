package ai

import (
	"context"
	"errors"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// Варианты первого проверяемого результата: 2–3 штуки, срок 2–8 недель.
const (
	minOptions = 2
	maxOptions = 3
	minWeeks   = 2
	maxWeeks   = 8
)

// ResultOptions: backend (openai → повтор → mock) + finishOptions (чистка, guardOptions, добивка из mock).
func (f *fallback) ResultOptions(ctx context.Context, draft, industry string, qs []model.Question) (model.ResultOptionsResult, error) {
	return run(f, func(b backend) (model.ResultOptionsResult, error) {
		r, err := b.resultOptions(ctx, draft, industry, qs)
		if err != nil {
			return r, err
		}
		return model.ResultOptionsResult{Options: finishOptions(r.Options, draft, industry, qs)}, nil
	})
}

func (c *openAIClient) resultOptions(ctx context.Context, draft, industry string, qs []model.Question) (model.ResultOptionsResult, error) {
	var res model.ResultOptionsResult
	if err := c.callWith(ctx, c.fastModel, "result_options", PromptResultOptions, cardUserMessage(draft, industry, qs), resultOptionsSchema(), &res); err != nil {
		return model.ResultOptionsResult{}, err
	}
	if len(res.Options) == 0 {
		return model.ResultOptionsResult{}, errors.New("openai: нет вариантов результата в ответе")
	}
	return res, nil
}

func (mockClient) resultOptions(_ context.Context, draft, _ string, qs []model.Question) (model.ResultOptionsResult, error) {
	return model.ResultOptionsResult{Options: mockOptions(draft, qs)}, nil
}

// mockOptions — два детерминированных варианта по ответам на data / users / need:
// малый «обзор одного процесса» и средний «прототип формы». Неизвестное названо неизвестным.
func mockOptions(draft string, qs []model.Question) []model.ResultOption {
	data, users, need := answerFor(qs, model.FieldData), answerFor(qs, model.FieldUsers), answerFor(qs, model.FieldNeed)
	if need == "" {
		need = needFragment(draft)
	}
	if need == "" {
		need = firstWords(draft, 12)
	}
	need = strings.TrimRight(need, ".!?;, ")

	byData, needData := "по примерам, которые вы передадите", "Какие данные или примеры есть — пока неизвестно, нужно уточнить."
	if data != "" {
		byData, needData = "по вашим данным ("+lowerFirst(data)+")", "Данные: "+data+"."
	}
	who, whoTitle := "будущие пользователи", "пользователей"
	if users != "" {
		who, whoTitle = lowerFirst(users), lowerFirst(firstWords(users, 3))
	}
	return []model.ResultOption{
		{
			Title:  "Обзор одного процесса",
			Result: "Команда разберёт один процесс " + byData + " и соберёт наглядный обзор: где теряется время и что можно упростить.",
			Check:  "Вы открываете обзор и на реальном примере из своей работы находите нужные сведения без ручного поиска.",
			Needs:  needData + " Один созвон, чтобы выбрать процесс.",
			Weeks:  2,
		},
		{
			Title:  "Прототип формы для " + whoTitle,
			Result: "Кликабельный прототип формы, в которой " + who + " вносят и находят записи. Цель прототипа: " + lowerFirst(need) + ".",
			Check:  upperFirst(who) + " проходят основной сценарий в прототипе без подсказки и подтверждают, что он решает задачу.",
			Needs:  "Время, чтобы " + who + " попробовали прототип и дали отзыв. " + needData,
			Weeks:  5,
		},
	}
}

// answerFor — первый непустой ответ на вопрос о поле k, без хвостовой пунктуации.
func answerFor(qs []model.Question, k model.FieldKey) string {
	for _, q := range qs {
		if a := strings.TrimRight(strings.TrimSpace(q.Answer), ".!?;, "); q.FieldKey == k && a != "" {
			return a
		}
	}
	return ""
}

func lowerFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	// аббревиатуры и имена («CRM», «Excel») не трогаем: только если вторая буква строчная
	if n == 0 || !unicode.IsUpper(r) {
		return s
	}
	if r2, _ := utf8.DecodeRuneInString(s[n:]); unicode.IsUpper(r2) {
		return s
	}
	return string(unicode.ToLower(r)) + s[n:]
}

func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

// guardOptions — защита от выдумок: вариант удаляется, если в title/result/check/needs есть число не из
// источника (в той же паре со словом, см. numPair) или имя собственное / бренд не из источника (inventedName).
func guardOptions(opts []model.ResultOption, sourceText string) []model.ResultOption {
	src := sourceIndex(sourceText)
	out := make([]model.ResultOption, 0, len(opts))
next:
	for _, o := range opts {
		for _, v := range []string{o.Title, o.Result, o.Check, o.Needs} {
			if inventedName(v, src) || inventedNumber(v, src) {
				continue next
			}
		}
		out = append(out, o)
	}
	return out
}

func inventedNumber(v string, src map[string]bool) bool {
	ts := tokens(v)
	for i, t := range ts {
		if hasDigit(t) && !src[numPair(ts, i)] {
			return true
		}
	}
	return false
}

// finishOptions — общая постобработка в обоих режимах: пустые и повторы выкидываются, срок в 2–8,
// guardOptions, сортировка от малого к среднему, не больше 3; меньше 2 — дополняется из mock.
func finishOptions(in []model.ResultOption, draft, industry string, qs []model.Question) []model.ResultOption {
	var clean []model.ResultOption
	for _, o := range in {
		o = model.ResultOption{Title: strings.TrimSpace(o.Title), Result: strings.TrimSpace(o.Result),
			Check: strings.TrimSpace(o.Check), Needs: strings.TrimSpace(o.Needs), Weeks: min(max(o.Weeks, minWeeks), maxWeeks)}
		if o.Title == "" || o.Result == "" || o.Check == "" || o.Needs == "" || hasTitle(clean, o.Title) {
			continue
		}
		clean = append(clean, o)
	}
	out := guardOptions(clean, sourceText(draft, industry, qs))
	if len(out) > maxOptions {
		out = out[:maxOptions]
	}
	for _, o := range mockOptions(draft, qs) {
		if len(out) < minOptions && !hasTitle(out, o.Title) {
			out = append(out, o)
		}
	}
	slices.SortStableFunc(out, func(a, b model.ResultOption) int { return a.Weeks - b.Weeks })
	return out
}

func hasTitle(opts []model.ResultOption, title string) bool {
	return slices.ContainsFunc(opts, func(o model.ResultOption) bool { return strings.EqualFold(o.Title, title) })
}
