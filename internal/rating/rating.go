// Package rating — детерминированная прозрачная формула рейтинга готовности
// бизнес-задачи (0–100) по ТЗ §4. Никакого ML: только длина текста и ключевые слова.
package rating

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// Weight — показатель рейтинга и его вес по ТЗ §4.
type Weight struct {
	Key    string
	Label  string
	Weight int
}

// Weights — 7 показателей в фиксированном порядке (порядок breakdown). Сумма = 100.
var Weights = []Weight{
	{"context_need", "Контекст и потребность", 20},
	{"data", "Данные и материалы", 20},
	{"expected_result", "Ожидаемый результат", 15},
	{"success_criteria", "Критерии успеха", 15},
	{"constraints", "Ограничения", 10},
	{"users", "Пользователи", 10},
	{"business_link", "Связь с бизнесом", 10},
}

// indicatorFields — какие поля карточки входят в показатель.
var indicatorFields = map[string][]model.FieldKey{
	"context_need":     {model.FieldContext, model.FieldNeed},
	"data":             {model.FieldData},
	"expected_result":  {model.FieldExpectedResult},
	"success_criteria": {model.FieldSuccessCriteria},
	"constraints":      {model.FieldConstraints},
	"users":            {model.FieldUsers},
	"business_link":    {model.FieldContact, model.FieldInteractionFormat},
}

// Keywords — единая таблица признаков «полноты» по полям. Поиск регистронезависимый,
// по подстроке в нормализованном тексте (ё→е, знаки препинания → пробел, по краям пробел).
// Пробел в начале/конце ключа означает границу слова (" до " не совпадёт с «доступ»).
var Keywords = map[model.FieldKey][]string{
	model.FieldSuccessCriteria: {"%", "процент", "срок", "числ", "снижени", "сниз", "сократ", " рост", "увелич",
		"не менее", "не более", "дней", "недел", "месяц", "минут"},
	model.FieldData: {"данн", "пример", "источник", "файл", "выгруз", "crm", "таблиц", "excel", "csv", "api",
		" баз", "датасет", "архив"},
	model.FieldConstraints: {"срок", "дедлайн", " дат", "бюджет", "технолог", "стек", "доступ", "нельзя", "только",
		" до ", "огранич", "конфиденц", "персональн"},
	model.FieldUsers: {"сотрудник", "клиент", "менеджер", "студент", "врач", "пользовател", "отдел", "компани",
		"покупател", "пациент", "оператор", "руководител", "специалист", "преподават", "аналитик", "бухгалтер"},
	model.FieldContact: {"@", "t.me"},
	model.FieldInteractionFormat: {"встреч", "созвон", "звон", "раз в", "еженедельн", "ежедневн", "ежемесячн",
		"чат", "email", "e-mail", "почт", "консультац", "zoom", "telegram", "телеграм", "обратн", "митинг"},
	model.FieldExpectedResult: {"прототип", "отчет", "модель", "сервис", "приложени", "дашборд", "скрипт",
		"алгоритм", "mvp", "документ", "презентац", "сайт", " бот", "-бот"},
}

// fieldRule — правило «полноты» поля: длина (если задана) И признак (если задан).
type fieldRule struct {
	minRunes int    // ≥ minRunes символов …
	minWords int    // … или ≥ minWords слов (0 = не проверяется)
	sign     string // чего не хватает, если нет признака (для reason)
	hint     string // что дописать (для missing)
}

var rules = map[model.FieldKey]fieldRule{
	model.FieldContext: {minRunes: 40,
		hint: "опишите, что происходит сейчас: процесс, масштаб, в чём проблема"},
	model.FieldNeed: {minRunes: 40,
		hint: "опишите, что именно нужно изменить или автоматизировать"},
	model.FieldData: {minRunes: 40, minWords: 6, sign: "нет упоминания данных или источника",
		hint: "укажите, какие данные доступны: выгрузки, таблицы, CRM, API, примеры, объём"},
	model.FieldExpectedResult: {minWords: 6, sign: "нет конкретного артефакта",
		hint: "назовите артефакт (прототип, сервис, дашборд, модель, отчёт, MVP) и что он делает"},
	model.FieldSuccessCriteria: {minRunes: 40, minWords: 6, sign: "нет измеримого признака: добавьте число или срок",
		hint: "добавьте измеримый критерий: число, %, срок (например, «снизить время обработки на 30% за 2 месяца»)"},
	model.FieldConstraints: {minRunes: 40, minWords: 6, sign: "не указаны сроки, бюджет, технологии или доступы",
		hint: "укажите сроки, бюджет, технологии, доступы или что делать нельзя"},
	model.FieldUsers: {minRunes: 40, minWords: 6, sign: "не указана роль пользователей",
		hint: "укажите, кто будет пользоваться решением: роли, отделы, клиенты, их количество"},
	model.FieldContact: {sign: "нет email, телефона или Telegram",
		hint: "оставьте email, телефон (+7…) или Telegram контактного лица"},
	model.FieldInteractionFormat: {sign: "не указан формат и частота общения",
		hint: "укажите формат и частоту: еженедельный созвон, чат, консультации, порядок обратной связи"},
}

type state int

const (
	stEmpty state = iota
	stPartial
	stFull
)

type fieldResult struct {
	st     state
	reason string // почему не полно (для stPartial)
}

// Compute считает рейтинг карточки. confirmed на формулу не влияет: подтверждение
// определяет публикацию, а баллы считаются одинаково (до подтверждения фронт
// подписывает их как «предварительные»).
func Compute(f model.Fields, confirmed bool) model.Rating {
	_ = confirmed
	r := model.Rating{Breakdown: make([]model.BreakdownItem, 0, len(Weights)), Missing: []model.MissingItem{}}
	for _, w := range Weights {
		earned, reason, hint := evalIndicator(w, f)
		r.Score += earned
		r.Breakdown = append(r.Breakdown, model.BreakdownItem{Key: w.Key, Label: w.Label, Weight: w.Weight, Earned: earned, Reason: reason})
		if earned < w.Weight {
			r.Missing = append(r.Missing, model.MissingItem{Key: w.Key, Label: w.Label, Gain: w.Weight - earned, Hint: hint})
		}
	}
	sort.SliceStable(r.Missing, func(i, j int) bool { return r.Missing[i].Gain > r.Missing[j].Gain })
	r.Level = LevelFor(r.Score)
	r.LevelLabel = model.LevelLabels[r.Level]
	return r
}

// LevelFor — шкала уровней ТЗ §4: 0–39 draft, 40–69 working, 70–89 ready, 90–100 priority.
func LevelFor(score int) model.Level {
	switch {
	case score >= 90:
		return model.LevelPriority
	case score >= 70:
		return model.LevelReady
	case score >= 40:
		return model.LevelWorking
	default:
		return model.LevelDraft
	}
}

func evalIndicator(w Weight, f model.Fields) (earned int, reason, hint string) {
	keys := indicatorFields[w.Key]
	var (
		nEmpty, nFull int
		reasons       []string
		hints         []string
	)
	for _, k := range keys {
		res := evalField(k, f[k])
		label := model.FieldLabels[k]
		switch res.st {
		case stEmpty:
			nEmpty++
			if len(keys) > 1 {
				reasons = append(reasons, fmt.Sprintf("«%s» не заполнено", label))
			}
			hints = append(hints, rules[k].hint)
		case stPartial:
			if len(keys) > 1 {
				reasons = append(reasons, fmt.Sprintf("«%s»: %s", label, res.reason))
			} else {
				reasons = append(reasons, res.reason)
			}
			hints = append(hints, rules[k].hint)
		case stFull:
			nFull++
		}
	}
	hint = upperFirst(strings.Join(hints, "; "))
	switch {
	case nFull == len(keys):
		return w.Weight, "Заполнено полностью", ""
	case nEmpty == len(keys):
		return 0, "Не заполнено", hint
	default:
		return w.Weight / 2, "Заполнено частично: " + strings.Join(reasons, "; "), hint
	}
}

func evalField(k model.FieldKey, raw string) fieldResult {
	text := strings.TrimSpace(raw)
	if text == "" {
		return fieldResult{st: stEmpty}
	}
	rule := rules[k]
	if !lengthOK(text, rule) {
		return fieldResult{st: stPartial, reason: "слишком коротко: " + lengthRequirement(rule)}
	}
	if kws := Keywords[k]; len(kws) > 0 && !hasSign(k, text, kws) {
		return fieldResult{st: stPartial, reason: rule.sign}
	}
	return fieldResult{st: stFull}
}

func lengthOK(text string, rule fieldRule) bool {
	if rule.minRunes == 0 && rule.minWords == 0 {
		return true
	}
	if rule.minRunes > 0 && utf8.RuneCountInString(text) >= rule.minRunes {
		return true
	}
	return rule.minWords > 0 && len(strings.Fields(text)) >= rule.minWords
}

func lengthRequirement(rule fieldRule) string {
	switch {
	case rule.minRunes > 0 && rule.minWords > 0:
		return fmt.Sprintf("нужно ≥ %d символов или ≥ %d слов", rule.minRunes, rule.minWords)
	case rule.minRunes > 0:
		return fmt.Sprintf("нужно ≥ %d символов", rule.minRunes)
	default:
		return fmt.Sprintf("нужно ≥ %d слов", rule.minWords)
	}
}

func hasSign(k model.FieldKey, text string, kws []string) bool {
	norm := normalize(text)
	for _, kw := range kws {
		if strings.Contains(norm, kw) {
			return true
		}
	}
	switch k {
	case model.FieldSuccessCriteria:
		return strings.IndexFunc(text, unicode.IsDigit) >= 0
	case model.FieldContact:
		return isPhone(text)
	}
	return false
}

// normalize: нижний регистр, ё→е, всё кроме букв/цифр и «%@.+-/» → пробел, пробелы по краям.
func normalize(s string) string {
	var b strings.Builder
	b.WriteByte(' ')
	for _, r := range strings.ToLower(s) {
		switch {
		case r == 'ё':
			b.WriteRune('е')
		case unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("%@.+-/", r):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	b.WriteByte(' ')
	return b.String()
}

// isPhone: ≥ 10 цифр и номер начинается с +7, 7 или 8.
func isPhone(s string) bool {
	var digits []rune
	for _, r := range s {
		if unicode.IsDigit(r) {
			digits = append(digits, r)
		}
	}
	return len(digits) >= 10 && (digits[0] == '7' || digits[0] == '8')
}

func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

// FormulaDescription — текст формулы для README.
func FormulaDescription() string {
	var b strings.Builder
	b.WriteString("Рейтинг готовности задачи = сумма баллов 7 показателей (0–100):\n")
	for _, w := range Weights {
		labels := make([]string, 0, 2)
		for _, k := range indicatorFields[w.Key] {
			labels = append(labels, "«"+model.FieldLabels[k]+"»")
		}
		fmt.Fprintf(&b, "- %s — %d баллов (поля %s)\n", w.Label, w.Weight, strings.Join(labels, " + "))
	}
	b.WriteString(`
Каждый показатель получает 0, половину (округление вниз) или полный вес:
- поле пустое → 0;
- заполнено, но не проходит проверку полноты → 50%;
- заполнено полно → 100%.
Для составных показателей (контекст + потребность, контакт + формат взаимодействия): оба пустые → 0, оба полные → 100%, иначе → 50%.

Проверка полноты (без ML, ключевые слова ищутся регистронезависимо по подстроке):
- Контекст, Потребность — не короче 40 символов;
- Данные — ≥ 40 символов или ≥ 6 слов И упоминание данных/примеров/источника (выгрузка, CRM, таблица, API, база, файл…);
- Ожидаемый результат — ≥ 6 слов И конкретный артефакт (прототип, сервис, дашборд, модель, отчёт, MVP…);
- Критерии успеха — ≥ 40 символов или ≥ 6 слов И измеримый признак (цифра, %, срок, снижение, рост, не менее…);
- Ограничения — ≥ 40 символов или ≥ 6 слов И сроки/дата/бюджет/технологии/доступы/«нельзя»/«только»/«до»;
- Пользователи — ≥ 40 символов или ≥ 6 слов И роль (сотрудник, клиент, менеджер, студент, врач, отдел…);
- Контакт — email, Telegram (@, t.me) или телефон (+7/8, ≥ 10 цифр);
- Формат взаимодействия — встречи, созвоны, «раз в…», еженедельно, чат, email, консультации.

Уровни: 0–39 — черновик, 40–69 — рабочая, 70–89 — готовая, 90–100 — приоритетная.
Подтверждение карточки не меняет формулу: оно публикует задачу; до подтверждения балл предварительный.
`)
	return b.String()
}
