package ai

import (
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// guardThreshold — доля значимых слов поля, которые должны встретиться в исходном тексте.
const guardThreshold = 0.6

// Guard — защита от выдуманных фактов (ТЗ §5). Возвращает копию со всеми 10 ключами,
// где каждое непустое поле (включая title) сохранено, только если ≥ 60% его значимых слов
// (длина ≥ 4, грубый стем — первые 5 букв) встречаются в sourceText, а каждое число стоит
// в источнике рядом с тем же словом (см. supported). Пустой title заполняет finishCard.
func Guard(fields model.Fields, sourceText string) model.Fields {
	src := map[string]bool{}
	ts := tokens(sourceText)
	for i, t := range ts {
		src[t] = true
		src[stem(t)] = true
		if r := []rune(t); len(r) > 4 {
			src[string(r[:4])] = true // слово из 4 букв («учёт») совпадает с формой «учёта»
		}
		if hasDigit(t) { // в источнике — обе пары: с соседом справа и слева
			src[numPair(ts, i)] = true
			if i > 0 {
				src[numPair(ts[:i+1], i)] = true
			}
		}
	}
	out := fields.Full()
	for _, k := range model.FieldKeys {
		v := strings.TrimSpace(out[k])
		out[k] = v
		if v != "" && !supported(v, src) {
			out[k] = ""
		}
	}
	return out
}

// numPair — ключ «число + следующее слово (стем)»; для числа в конце текста —
// «предыдущее слово (стем) + число»; для одиночного числа — само число.
func numPair(ts []string, i int) string {
	switch {
	case i+1 < len(ts):
		return "#" + ts[i] + " " + stem(ts[i+1])
	case i > 0:
		return "#" + stem(ts[i-1]) + " " + ts[i]
	}
	return ts[i]
}

// supported: каждое число поля стоит в источнике в той же паре со словом (numPair) —
// «2» из «2 недели» нельзя перенести в «до 2 операторов», — и ≥ 60% слов длиной ≥ 4
// совпадают по стему.
func supported(v string, src map[string]bool) bool {
	var total, hit int
	ts := tokens(v)
	for i, t := range ts {
		switch {
		case hasDigit(t):
			if !src[numPair(ts, i)] {
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

// LastCall — последний реальный вызов сборки карточки: вход, сырой ответ модели, что вырезал Guard.
// Показывается на GET /api/ai как наблюдаемое доказательство работы AI и защиты от выдумок (ТЗ §5).
type LastCall struct {
	At             time.Time        `json:"at"`
	Industry       string           `json:"industry"`
	Draft          string           `json:"draft"`
	Questions      []model.Question `json:"questions"`
	RawFields      model.Fields     `json:"raw_fields"`
	Fields         model.Fields     `json:"fields"`
	RemovedByGuard []model.FieldKey `json:"removed_by_guard"`
}

var (
	lastMu   sync.Mutex
	lastCall *LastCall // ponytail: один процесс — package-level достаточно
)

func lastCallCopy() *LastCall {
	lastMu.Lock()
	defer lastMu.Unlock()
	if lastCall == nil {
		return nil
	}
	c := *lastCall
	return &c
}

// finishCard — общая постобработка карточки в обоих режимах.
func finishCard(fields model.Fields, draft, industry string, qs []model.Question) model.CardResult {
	raw := fields.Full()
	out := Guard(fields, sourceText(draft, industry, qs))
	if out[model.FieldTitle] == "" {
		out[model.FieldTitle] = firstWords(draft, 8)
	}
	removed := []model.FieldKey{}
	for _, k := range model.FieldKeys {
		if raw[k] != "" && out[k] == "" {
			removed = append(removed, k)
		}
	}
	lastMu.Lock()
	lastCall = &LastCall{At: time.Now(), Industry: industry, Draft: draft, Questions: qs, RawFields: raw, Fields: out.Full(), RemovedByGuard: removed}
	lastMu.Unlock()
	return model.CardResult{Fields: out}
}

// titleStopWords — служебные слова, на которых название не должно обрываться.
var titleStopWords = map[string]bool{"в": true, "на": true, "с": true, "и": true, "а": true, "но": true,
	"для": true, "по": true, "к": true, "у": true, "о": true, "от": true, "до": true, "из": true,
	"что": true, "как": true, "все": true, "всё": true, "без": true}

// firstWords — первые n слов без хвостовых предлогов/союзов/частиц и завершающих «,», «—», «:».
func firstWords(s string, n int) string {
	w := strings.Fields(s)
	if len(w) > n {
		w = w[:n]
	}
	for len(w) > 0 {
		last := strings.TrimRight(w[len(w)-1], ",;:-–—")
		if last != "" && (len(w) == 1 || !titleStopWords[strings.ToLower(last)]) {
			w[len(w)-1] = last
			break
		}
		w = w[:len(w)-1]
	}
	return strings.Join(w, " ")
}
