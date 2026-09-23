package ai

import (
	"strings"
	"unicode"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// guardThreshold — доля значимых слов поля, которые должны встретиться в исходном тексте.
const guardThreshold = 0.6

// Guard — защита от выдуманных фактов (ТЗ §5). Возвращает копию со всеми 10 ключами,
// где каждое непустое поле, кроме title, сохранено, только если ≥ 60% его значимых слов
// (длина ≥ 4, грубый стем — первые 5 букв) и все числа встречаются в sourceText.
func Guard(fields model.Fields, sourceText string) model.Fields {
	src := map[string]bool{}
	for _, t := range tokens(sourceText) {
		src[t] = true
		src[stem(t)] = true
	}
	out := fields.Full()
	for _, k := range model.FieldKeys {
		v := strings.TrimSpace(out[k])
		out[k] = v
		if k == model.FieldTitle || v == "" {
			continue
		}
		if !supported(v, src) {
			out[k] = ""
		}
	}
	return out
}

// supported: все числа поля есть в источнике и ≥ 60% слов длиной ≥ 4 совпадают по стему.
func supported(v string, src map[string]bool) bool {
	var total, hit int
	for _, t := range tokens(v) {
		switch {
		case hasDigit(t):
			if !src[t] {
				return false
			}
		case len([]rune(t)) >= 4:
			total++
			if src[stem(t)] {
				hit++
			}
		}
	}
	return total == 0 || float64(hit) >= guardThreshold*float64(total)
}

func tokens(s string) []string {
	s = strings.ReplaceAll(strings.ToLower(s), "ё", "е")
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

func stem(t string) string {
	r := []rune(t)
	if len(r) > 5 {
		r = r[:5]
	}
	return string(r)
}

func hasDigit(t string) bool {
	return strings.IndexFunc(t, unicode.IsDigit) >= 0
}

// sourceText — всё, что сообщил пользователь: отрасль, черновик и ответы.
func sourceText(draft, industry string, qs []model.Question) string {
	parts := []string{industry, draft}
	for _, q := range qs {
		parts = append(parts, q.Answer)
	}
	return strings.Join(parts, "\n")
}

// finishCard — общая постобработка карточки в обоих режимах.
func finishCard(fields model.Fields, draft, industry string, qs []model.Question) model.CardResult {
	out := Guard(fields, sourceText(draft, industry, qs))
	if out[model.FieldTitle] == "" {
		out[model.FieldTitle] = firstWords(draft, 8)
	}
	return model.CardResult{Fields: out}
}

func firstWords(s string, n int) string {
	w := strings.Fields(s)
	if len(w) > n {
		w = w[:n]
	}
	return strings.Join(w, " ")
}
