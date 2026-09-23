// Package rating — детерминированная прозрачная формула рейтинга готовности
// бизнес-задачи (0–100) по ТЗ §4. Никакого ML: только ключевые слова, простые
// шаблоны (email, телефон, число с единицей) и длина как вспомогательный сигнал.
package rating

import (
	"fmt"
	"regexp"
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

// Keywords — основной признак «полноты» по полям. Поиск регистронезависимый,
// по подстроке в нормализованном тексте (ё→е, знаки препинания → пробел, по краям пробел).
// Пробел в начале/конце ключа означает границу слова (" до " не совпадёт с «доступ»).
// Вторые признаки (передача данных, ритм общения, проверка результата…) — ниже.
var Keywords = map[model.FieldKey][]string{
	// Метрика или единица рядом с числом (см. measuredNumber).
	model.FieldSuccessCriteria: {"%", "процент", "минут", " мин ", "час", " ч ", " дн", " день", "недел", "месяц",
		" мес ", " лет ", " год", "заяв", "клиент", "сотрудник", "пользовател", "обращен", "звонк", "ошиб",
		"секунд", " сек ", "руб", "тенге", " тг ", " тыс", " млн", " шт", " раз ", " раза ", "балл", "человек", " чел "},
	// Материал: что именно есть.
	model.FieldData: {" данн", "выгруз", "таблиц", "файл", "crm", "api", " баз", "образц", "пример", "интервью",
		" лог", "excel", "csv", "датасет", "архив", "документ", "записи", "журнал"},
	model.FieldConstraints: {"срок", "дедлайн", " дат", "бюджет", "технолог", "стек", "доступ", "нельзя", "не использ", "только",
		" до ", "огранич", "конфиденц", "персональн"},
	model.FieldUsers: {"сотрудник", "клиент", "менеджер", "студент", "врач", "пользовател", "отдел", "компани",
		"покупател", "пациент", "оператор", "руководител", "специалист", "преподават", "аналитик", "бухгалтер",
		"инженер", "юрист", "продав", "кассир", "водител", "учител", "диспетчер", "логист", "кладовщ",
		"администратор", "персонал", "рекрутер", " hr ", "мастер", "курьер", "агент", "слушател", "методист"},
	// Контакт проверяется шаблонами (email / Telegram / телефон), см. contactUsable.
	model.FieldContact: {"@", "t.me/"},
	// Канал общения; полный балл — вместе с ритмом или сроком ответа (formatRhythm).
	model.FieldInteractionFormat: {"созвон", "встреч", "zoom", "meet", "чат", "telegram", "телеграм", "email",
		"e-mail", "почт", "звон", "митинг", "teams", "whatsapp", "slack", "консультац"},
	// Артефакт; полный балл — вместе с описанием функции (resultFunction) или развёрнутым описанием.
	model.FieldExpectedResult: {"прототип", "отчет", "модель", "сервис", "приложени", "дашборд", "скрипт",
		"алгоритм", "mvp", "документ", "презентац", "сайт", " бот", "-бот", "система", "интеграц", "панель",
		"api", "парсер", "калькулятор", "витрин"},
}

var (
	// successVerify — описание проверки/приёмки вместо числа.
	successVerify = []string{"проверим", "проверяем", "принимаем если", "примем если", "принимается если",
		"считается выполненным", "считаем выполненным", "тест", "сценари", " пример", " на примере"}
	// successComparators — слова перед числом, делающие его целевым значением.
	successComparators = []string{"менее", "более", "минимум", "максимум", "до", "от", "≥", "≤", "<", ">"}
	// successChange — глагол изменения: тогда «на N» / «в N» / «с N» — целевое значение.
	successChange = []string{"сниз", "снижен", "сократ", "рост", "вырас", "увелич", "уменьш", "повыс", "ускор"}
	// dataTransfer — способ или срок передачи материалов.
	dataTransfer = []string{"передад", "передади", "дадим доступ", "в первую неделю", "после nda", "по запросу",
		" доступ ", " доступ к", " доступа ", " доступны", " доступно", " доступен", " подготовлен", "предостав", "в течение", "отправим", "пришлем", "выдадим", "загрузим"}
	// dataNegative — явное отсутствие материалов.
	dataNegative = []string{"нет данных", "данных нет", "недоступн", "не найден", "отсутству", "нет доступа",
		"нет выгруз", "пока нет", "не собира", "не ведем", "не ведется"}
	// dataPlan — план получения, если материалов пока нет.
	dataPlan = []string{"передад", "предостав", "дадим", "получим", "соберем", "запросим", "выгрузим",
		"после nda", "в течение", "в первую неделю", "по запросу", "к началу"}
	// formatRhythm — ритм общения или срок ответа.
	formatRhythm = []string{"раз в", "еженедельн", "ежедневн", "ежемесячн", "каждую", "каждый", "каждые", "каждое",
		"по понедельникам", "по вторникам", "по средам", "по четвергам", "по пятницам", "в течение",
		"не позднее", " дня", " дней", "часов", " часа", "недел"}
	// resultFunction — что делает артефакт.
	resultFunction = []string{"котор", " для ", " с ", " по ", "позволя", "показыва", "умеет", "автоматическ",
		"считает", "рассчитыва", "формирует", "классифиц", "прогноз", "фильтр", "поиск", "уведомл", "отправля",
		"распредел", "анализ", "отслежива", "собирает", "назначает"}
	// stubValues — явные заглушки (после нормализации пробелов и регистра, без концевой пунктуации).
	stubValues = map[string]bool{"x": true, "xx": true, "xxx": true, "х": true, "хх": true, "ххх": true,
		"-": true, "—": true, ".": true, "?": true, "todo": true, "tbd": true, "n/a": true, "na": true,
		"нет": true, "не знаю": true, "нет данных": true, "пока нет": true, "позже": true, "заполню": true}
	// contactStubs — дополнительные заглушки для контакта.
	contactStubs = map[string]bool{"@": true, "tg": true, "тг": true}

	reEmail    = regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.]+`)
	reTelegram = regexp.MustCompile(`@[A-Za-z0-9_]{5,}|t\.me/\S+`)
	rePhone    = regexp.MustCompile(`\+?\d[\d\s().-]{8,}\d`)
	reVersion  = regexp.MustCompile(`(верси\S*|релиз\S*|v|№)\s*\d+(\.\d+)*`)
)

// fieldRule — подсказки поля: что дописать (для missing).
type fieldRule struct {
	hint string
}

var rules = map[model.FieldKey]fieldRule{
	model.FieldContext:           {hint: "опишите, что происходит сейчас: процесс, масштаб, в чём проблема"},
	model.FieldNeed:              {hint: "опишите, что именно нужно изменить или автоматизировать"},
	model.FieldData:              {hint: "укажите, какие материалы есть (выгрузка, таблицы, CRM, API, примеры, интервью) и как и когда передадите доступ"},
	model.FieldExpectedResult:    {hint: "назовите артефакт (прототип, сервис, дашборд, отчёт, бот) и что он делает"},
	model.FieldSuccessCriteria:   {hint: "как проверим результат: число с метрикой («снизить время обработки на 30%») или сценарий приёмки («принимаем, если на 20 примерах…»)"},
	model.FieldConstraints:       {hint: "укажите сроки, бюджет, технологии, доступы или что делать нельзя"},
	model.FieldUsers:             {hint: "укажите, кто будет пользоваться решением: роль, отдел или тип клиентов и их количество"},
	model.FieldContact:           {hint: "оставьте email, телефон (10+ цифр) или Telegram (@username, t.me/…) контактного лица"},
	model.FieldInteractionFormat: {hint: "укажите канал и ритм: «созвон раз в неделю», «чат в Telegram, отвечаем в течение дня»"},
}

type state int

const (
	stEmpty state = iota
	stStub
	stPartial
	stFull
)

type fieldResult struct {
	st     state
	reason string // почему не полно (для stPartial)
	hint   string // что именно дописать (если пусто — rules[k].hint)
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
		nEmpty, nStub, nFull int
		reasons              []string
		hints                []string
	)
	for _, k := range keys {
		res := evalField(k, f[k])
		label := model.FieldLabels[k]
		h := res.hint
		if h == "" {
			h = rules[k].hint
		}
		switch res.st {
		case stEmpty, stStub:
			nEmpty++
			if res.st == stStub {
				nStub++
			}
			if len(keys) > 1 {
				what := "не заполнено"
				if res.st == stStub {
					what = "заглушка вместо ответа"
				}
				reasons = append(reasons, fmt.Sprintf("«%s» %s", label, what))
			}
			hints = append(hints, h)
		case stPartial:
			if len(keys) > 1 {
				reasons = append(reasons, fmt.Sprintf("«%s»: %s", label, res.reason))
			} else {
				reasons = append(reasons, res.reason)
			}
			hints = append(hints, h)
		case stFull:
			nFull++
		}
	}
	hint = upperFirst(strings.Join(hints, "; "))
	switch {
	case nFull == len(keys):
		return w.Weight, "Заполнено полностью", ""
	case nEmpty == len(keys) && nStub > 0:
		return 0, "Не заполнено (заглушка)", hint
	case nEmpty == len(keys):
		return 0, "Не заполнено", hint
	default:
		return w.Weight / 2, "Заполнено частично: " + strings.Join(reasons, "; "), hint
	}
}

func evalField(k model.FieldKey, raw string) fieldResult {
	text := strings.Join(strings.Fields(raw), " ") // пробелы и переносы не влияют на оценку
	if text == "" {
		return fieldResult{st: stEmpty}
	}
	if k == model.FieldContact && contactUsable(text) {
		return fieldResult{st: stFull} // телефон состоит из цифр — проверяем до правила «без букв»
	}
	if isStub(k, text) {
		return fieldResult{st: stStub}
	}
	norm := normalize(text)
	switch k {
	case model.FieldContext:
		if uniqueRunes(norm) < 40 {
			return fieldResult{st: stPartial, reason: "опишите, что происходит сейчас и что нужно изменить"}
		}
	case model.FieldNeed:
		if uniqueRunes(norm) < 40 {
			return fieldResult{st: stPartial, reason: "опишите, что происходит сейчас и что нужно изменить"}
		}
	case model.FieldUsers:
		if !hasAny(norm, Keywords[k]) {
			return fieldResult{st: stPartial, reason: "не указана роль пользователей"}
		}
	case model.FieldExpectedResult:
		if !hasAny(norm, Keywords[k]) {
			return fieldResult{st: stPartial, reason: "нет конкретного артефакта"}
		}
		if !hasAny(norm, resultFunction) && len(uniqueWords(norm)) < 6 {
			return fieldResult{st: stPartial, reason: "назван артефакт, но не сказано, что он делает",
				hint: "допишите, что делает артефакт и для кого (например, «отчёт по продажам с фильтром по датам»)"}
		}
	case model.FieldSuccessCriteria:
		if !measuredNumber(norm) && !hasAny(norm, successVerify) {
			return fieldResult{st: stPartial, reason: "нет измеримого признака или способа проверки"}
		}
		if len(uniqueWords(norm)) < 3 {
			return fieldResult{st: stPartial, reason: "не сказано, что именно измеряем или проверяем"}
		}
	case model.FieldData:
		return evalData(norm)
	case model.FieldConstraints:
		if !hasAny(norm, Keywords[k]) {
			return fieldResult{st: stPartial, reason: "не указаны сроки, бюджет, технологии или доступы"}
		}
		if len(uniqueWords(norm)) < 3 {
			return fieldResult{st: stPartial, reason: "не ясно, что именно ограничено"}
		}
	case model.FieldContact:
		return fieldResult{st: stPartial, reason: "нет пригодного email, телефона или Telegram"}
	case model.FieldInteractionFormat:
		ch, rh := hasAny(norm, Keywords[k]), hasAny(norm, formatRhythm)
		switch {
		case ch && rh:
		case ch:
			return fieldResult{st: stPartial, reason: "указан канал, но нет ритма или срока ответа",
				hint: "добавьте ритм или срок ответа: «раз в неделю», «отвечаем в течение дня»"}
		case rh:
			return fieldResult{st: stPartial, reason: "указан ритм, но нет канала связи",
				hint: "добавьте канал: созвон, встреча, чат в Telegram, email"}
		default:
			return fieldResult{st: stPartial, reason: "не указаны канал и ритм общения"}
		}
	}
	return fieldResult{st: stFull}
}

func evalData(norm string) fieldResult {
	material := hasAny(norm, Keywords[model.FieldData])
	if hasAny(norm, dataNegative) && !hasAny(norm, dataPlan) {
		return fieldResult{st: stPartial, reason: "материалов пока нет и не указан план их получения",
			hint: "укажите, как и когда получим материалы: кто даст доступ, что соберём (выгрузка, образцы, интервью)"}
	}
	switch {
	case material && hasAny(norm, dataTransfer):
		return fieldResult{st: stFull}
	case material:
		return fieldResult{st: stPartial, reason: "не указано, как и когда передадите материалы",
			hint: "укажите способ и срок передачи: «передадим после NDA», «дадим доступ в первую неделю»"}
	default:
		return fieldResult{st: stPartial, reason: "нет упоминания материалов (выгрузка, таблицы, CRM, примеры)"}
	}
}

// isStub — заглушка: нет ни одной буквы, «x…», или значение из явного списка.
func isStub(k model.FieldKey, text string) bool {
	if strings.IndexFunc(text, unicode.IsLetter) < 0 {
		return true
	}
	v := strings.ReplaceAll(strings.ToLower(text), "ё", "е")
	v = strings.TrimRightFunc(v, func(r rune) bool { return unicode.IsPunct(r) && r != '?' || unicode.IsSpace(r) })
	if stubValues[v] || (k == model.FieldContact && contactStubs[v]) {
		return true
	}
	return strings.Trim(v, "xх .,!?-—") == "" // «xxxx», «х х х»
}

func contactUsable(text string) bool {
	if reEmail.MatchString(text) || reTelegram.MatchString(text) {
		return true
	}
	for _, m := range rePhone.FindAllString(text, -1) {
		n := 0
		for _, r := range m {
			if unicode.IsDigit(r) {
				n++
			}
		}
		if n >= 10 && n <= 15 {
			return true
		}
	}
	return false
}

// measuredNumber: есть число (не номер версии), рядом с которым стоит единица/метрика
// («30%», «10 минут», «50 заявок»), или перед которым стоит сравнение («не менее 85»,
// «до 10») либо «на/в/с N» при глаголе изменения («снизить … на 30»).
func measuredNumber(norm string) bool {
	norm = reVersion.ReplaceAllString(norm, " ")
	words := strings.Fields(norm)
	change := hasAny(norm, successChange)
	for i, w := range words {
		if strings.IndexFunc(w, unicode.IsDigit) < 0 {
			continue
		}
		tail := strings.TrimLeftFunc(w, func(r rune) bool { return unicode.IsDigit(r) || strings.ContainsRune(".,+-/", r) })
		window := " " + tail + " "
		for j := i + 1; j <= i+2 && j < len(words); j++ {
			window += words[j] + " "
		}
		if hasAny(window, Keywords[model.FieldSuccessCriteria]) {
			return true
		}
		for j := i - 1; j >= 0 && j >= i-2; j-- {
			if contains(successComparators, words[j]) {
				return true
			}
			if change && j == i-1 && (words[j] == "на" || words[j] == "в" || words[j] == "с") {
				return true
			}
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func hasAny(norm string, kws []string) bool {
	for _, kw := range kws {
		if strings.Contains(norm, kw) {
			return true
		}
	}
	return false
}

// uniqueWords — слова нормализованного текста без повторов (повтор фразы не добавляет смысла).
func uniqueWords(norm string) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range strings.Fields(norm) {
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

func uniqueRunes(norm string) int {
	return utf8.RuneCountInString(strings.Join(uniqueWords(norm), " "))
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
- поле пустое или заглушка (x, «-», «?», todo, «не знаю», «нет данных», текст без букв…) → 0;
- заполнено, но не хватает смысла (см. ниже) → 50%;
- заполнено полно → 100%.
Для составных показателей (контекст + потребность, контакт + формат взаимодействия): оба пустые → 0, оба полные → 100%, иначе → 50%.

Проверка полноты (без ML; пробелы и повторы слов не учитываются, ключевые слова — регистронезависимо):
- Контекст, Потребность — описание процесса и изменения, не короче 40 символов без повторов;
- Данные — назван материал (выгрузка, таблица, CRM, API, файлы, примеры, интервью, логи) И способ/срок передачи (передадим, доступ, после NDA, в первую неделю…); «данных нет / недоступно» без плана получения → 50%;
- Ожидаемый результат — артефакт (прототип, сервис, дашборд, отчёт, бот…) И что он делает (или развёрнутое описание);
- Критерии успеха — число вместе с метрикой/единицей («на 30%», «не более 10 минут», «50 заявок») ИЛИ описание проверки (проверим, принимаем если, тест, сценарий, пример); номер версии числом не считается; от 3 разных слов;
- Ограничения — сроки/дата/бюджет/технологии/доступы/«нельзя»/«только»/«до» и что именно ограничено (от 3 разных слов);
- Пользователи — названа роль (менеджер, сотрудник, клиент, врач, отдел…), длина не важна;
- Контакт — email, Telegram (@username от 5 символов, t.me/…) или телефон (от 10 цифр); иной текст → 50%, «@»/«tg» → 0;
- Формат взаимодействия — канал (созвон, встреча, чат, Telegram, email…) И ритм или срок ответа (раз в неделю, еженедельно, в течение дня…); одно из двух → 50%.

Уровни: 0–39 — черновик, 40–69 — рабочая, 70–89 — готовая, 90–100 — приоритетная.
Подтверждение карточки не меняет формулу: оно публикует задачу; до подтверждения балл предварительный.
`)
	return b.String()
}
