package ai

import (
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// guardThreshold — доля значимых слов поля, которые должны встретиться в исходном тексте.
const guardThreshold = 0.6

// Guard — защита от выдуманных фактов (ТЗ §5). Возвращает копию со всеми 10 ключами,
// где каждое непустое поле (включая title) сохранено, только если:
//   - ≥ 60% его значимых слов (длина ≥ 4) совпадают со словами sourceText по стему (см. stem);
//   - каждое число стоит в источнике рядом с тем же словом (по стему, см. supported);
//   - нет слова с заглавной буквы не в начале предложения (имя, бренд), которого нет в источнике;
//   - слово не сменило полярность: в поле «не передадим», а в источнике только «передадим»
//     (или наоборот, см. polarity).
//
// Пустой title заполняет finishCard.
func Guard(fields model.Fields, sourceText string) model.Fields {
	src := map[string]bool{}
	ts := tokens(sourceText)
	for i, t := range ts {
		src[t] = true
		src[stem(t)] = true
		// префиксы 4 и 5 букв: «таблицах» (стем из 5) совпадёт с «таблиц» (стем из 4) и наоборот
		for _, n := range []int{4, 5} {
			if r := []rune(t); len(r) >= n {
				src[trimEnding(r[:n])] = true
			}
		}
		if hasDigit(t) { // в источнике — обе пары: с соседом справа и слева
			src[numPair(ts, i)] = true
			if i > 0 {
				src[numPair(ts[:i+1], i)] = true
			}
		}
	}
	srcPol := polarity(sourceText)
	out := fields.Full()
	for _, k := range model.FieldKeys {
		v := strings.TrimSpace(out[k])
		out[k] = v
		if v != "" && (!supported(v, src) || inventedName(v, src) || polarityFlipped(polarity(v), srcPol)) {
			out[k] = ""
		}
	}
	return out
}

// numPair — ключ «число + следующее слово (стем)»; для числа в конце текста —
// «предыдущее слово (стем) + число»; для одиночного числа — само число.
// Стем соседа снимает окончание: «2 дня» = «2 дней», «4 часа» = «4 часов».
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

// inventedName — в поле есть слово с заглавной буквы не в начале предложения (имя
// собственное, бренд: «Kaspi», «Сбербанк»), которого нет в источнике даже по стему.
func inventedName(v string, src map[string]bool) bool {
	sentStart := true
	var word []rune
	check := func() bool {
		defer func() { word = word[:0] }()
		if len(word) == 0 {
			return false
		}
		first := sentStart
		sentStart = false
		if first || !unicode.IsUpper(word[0]) || hasDigit(string(word)) {
			return false
		}
		t := tokens(string(word))
		return len(t) == 1 && !src[t[0]] && !src[stem(t[0])]
	}
	for _, r := range v {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			word = append(word, r)
			continue
		}
		if check() {
			return true
		}
		if strings.ContainsRune(".!?…\n", r) {
			sentStart = true
		}
	}
	return check()
}

// negMarkers — маркеры отрицания для проверки полярности.
var negMarkers = map[string]bool{"не": true, "нет": true, "никак": true, "никаких": true, "без": true}

const (
	polPos = 1 << iota // слово встречается без отрицания
	polNeg             // слово встречается с отрицанием
)

// polarity — для стема каждого значимого слова (длина ≥ 4): встречается ли оно с отрицанием
// (маркер не дальше 2 слов перед ним или «нет» сразу после: «данных нет») и без него.
// Отрицание действует в пределах фрагмента между знаками препинания.
func polarity(text string) map[string]int {
	out := map[string]int{}
	clauses := strings.FieldsFunc(text, func(r rune) bool { return strings.ContainsRune(",.;:!?()—–\n", r) })
	for _, c := range clauses {
		ts := tokens(c)
		for i, t := range ts {
			if negMarkers[t] || hasDigit(t) || len([]rune(t)) < 4 {
				continue
			}
			bit := polPos
			if i+1 < len(ts) && ts[i+1] == "нет" {
				bit = polNeg
			}
			for j := max(0, i-2); j < i; j++ {
				if negMarkers[ts[j]] {
					bit = polNeg
				}
			}
			out[stem(t)] |= bit
		}
	}
	return out
}

// polarityFlipped — слово поля есть в источнике, но только с противоположной полярностью.
func polarityFlipped(field, src map[string]int) bool {
	for s, p := range field {
		if sp := src[s]; sp != 0 && sp&p == 0 {
			return true
		}
	}
	return false
}

func tokens(s string) []string {
	s = strings.ReplaceAll(strings.ToLower(s), "ё", "е")
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// stem — грубый стем: первые 4 буквы для слов длиной 4–6, первые 5 — для более длинных,
// без концевых гласных, «й» и «ь» (не короче 2 букв): «заявки»/«заявок» → «заяв»,
// «таблицах»/«таблице» → «табл», «дня»/«дней» → «дн», «часа»/«часов» → «час».
func stem(t string) string {
	r := []rune(t)
	switch {
	case len(r) > 6:
		r = r[:5]
	case len(r) >= 4:
		r = r[:4]
	}
	return trimEnding(r)
}

func trimEnding(r []rune) string {
	for len(r) > 2 && strings.ContainsRune("аеиоуыэюяйь", r[len(r)-1]) {
		r = r[:len(r)-1]
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

var (
	reEmail = regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.]+`)
	rePhone = regexp.MustCompile(`\+?\d[\d\s()-]{8,}\d`)
)

// maskPII — email и телефоны в публичном примере last_call заменяются на маску.
func maskPII(s string) string {
	s = reEmail.ReplaceAllString(s, "***@***")
	return rePhone.ReplaceAllString(s, "+***")
}

func lastCallCopy() *LastCall {
	lastMu.Lock()
	defer lastMu.Unlock()
	if lastCall == nil {
		return nil
	}
	c := *lastCall
	c.Draft = maskPII(c.Draft)
	c.Questions = append([]model.Question(nil), c.Questions...)
	for i := range c.Questions {
		c.Questions[i].Answer = maskPII(c.Questions[i].Answer)
	}
	c.RawFields, c.Fields = model.Fields{}, model.Fields{}
	for k, v := range lastCall.RawFields {
		c.RawFields[k] = maskPII(v)
	}
	for k, v := range lastCall.Fields {
		c.Fields[k] = maskPII(v)
	}
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
