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
		"администратор", "персонал", "рекрутер", " hr ", "мастер", "курьер", "агент", "слушател", "методист",
		"команд", "владел", "пекар", "продавц", "жильц", "приёмщ", "приемщ", "работник", "заказчик", "исполнител",
		"абитуриент", "ученик", "родител", "воспитател", "библиотекар", "посетител", "волонт", "стажёр", "стажер",
		"техник", "механик", "фермер", "поставщик", "партнёр", "партнер", "сотрудни", "жител"},
	// Контакт проверяется шаблонами (email / Telegram / телефон), см. contactKind.
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
		"считается выполненным", "считаем выполненным", "тест", "сценари", " пример", " на примере",
		"принимает", "принят", "проверк", "пилот", "приемк", " когда ", " если "}
	// successAbstract — абстрактная метрика: число есть, но не сказано, как её измерять.
	successAbstract = [][2]string{{"качеств", "качество"}, {"эффективн", "эффективность"}, {"удобств", "удобство"}}
	// successConcrete — конкретная метрика, при которой абстрактное слово не страшно.
	successConcrete = []string{"точност", "врем", "доля", "доли", "количеств", "число", "числа", "ошиб", "конверс", "nps", "оценк"}
	// successComparators — слова перед числом, делающие его целевым значением.
	successComparators = []string{"менее", "более", "минимум", "максимум", "до", "от", "≥", "≤", "<", ">"}
	// successChange — глагол изменения: тогда «на N» / «в N» / «с N» — целевое значение.
	successChange = []string{"сниз", "снижен", "сократ", "рост", "вырас", "увелич", "уменьш", "повыс", "ускор"}
	// dataTransfer — конкретный способ или срок передачи материалов (самодостаточные фразы).
	// «доступны/доступно» (наличие) планом не считается — только «дадим/предоставим доступ», «доступ к …».
	dataTransfer = []string{"дадим доступ", "предоставим доступ", "откроем доступ", "доступ к ", "в первую неделю",
		"после nda", "после подписан", "по запросу", " подготовлен", "в течение"}
	// dataTransferVerb — глагол передачи: засчитывается только вместе с конкретикой (dataTransferConcrete);
	// «передадим позже / потом / когда-нибудь» — не план.
	dataTransferVerb     = []string{"передад", "передади", "предостав", "отправим", "пришлем", "выдадим", "загрузим"}
	dataTransferConcrete = []string{"недел", " дн", " дня ", " дней", "месяц", " числа", "nda", "подписан", "выгруз",
		"файл", "api", "почт", "email", "ссылк", " диск", "архив", "excel", "csv", "январ", "феврал", "март", "апрел",
		" мая ", "июн", "июл", "август", "сентябр", "октябр", "ноябр", "декабр"}
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
	// stubWords / stubPairs — слова-заглушки внутри текста (по буквенным словам, нижний регистр, ё→е).
	stubWords = map[string]bool{"x": true, "xx": true, "xxx": true, "х": true, "хх": true, "ххх": true,
		"todo": true, "tbd": true, "позже": true, "заполню": true, "ok": true, "ок": true, "да": true, "тест": true}
	stubPairs = map[string]bool{"не знаю": true, "пока нет": true}
	// refusalMarkers — явный отказ (по строке буквенно-цифровых слов с пробелами по краям).
	refusalMarkers = []string{" нет ", " не будет ", " никаких ", " не передадим ", " не планируем ", "отсутству",
		" не ведем ", " не хотим ", " тоже нет "}
	// refusalAlt — план или альтернатива, при которых отказ не обнуляет поле (ищется после удаления «не/без X»).
	refusalAlt = []string{" но ", " вместо ", " зато ", " после ", " сможем ", "дадим доступ", "предоставим доступ", " обезличен", " передад",
		" предостав", " дадим", " получим", " соберем", " запросим", " выгрузим", " планируем", " в течение",
		" к началу", " по запросу"}

	reEmail    = regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.]+`)
	reTelegram = regexp.MustCompile(`@[A-Za-z0-9_]{5,}|t\.me/\S+`)
	rePhone    = regexp.MustCompile(`\+?\d[\d\s().-]{8,}\d`)
	reVersion  = regexp.MustCompile(`(верси\S*|релиз\S*|v|№)\s*\d+(\.\d+)*`)
	reNegated  = regexp.MustCompile(` (не|без) \S+| нет доступ\S*`)
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
	stEmpty   state = iota
	stStub          // заглушка целиком («x», «todo», текст без букв)
	stJunk          // нет признаков показателя и меньше 3 значимых слов («asdf», «x клиент»)
	stRefusal       // явный отказ без плана («данных нет»)
	stPartial
	stFull
)

// minMeaningful — минимум значимых слов, чтобы текст без признаков не считался заглушкой.
const minMeaningful = 3

const (
	reasonEmpty   = "Не заполнено"
	reasonStub    = "Не заполнено (заглушка)"
	reasonJunk    = "Не заполнено: нет сведений по показателю"
	reasonRefusal = "Сведения отсутствуют (отказ)"
)

type fieldResult struct {
	st     state
	reason string // stPartial — чего не хватает; stFull — какой признак найден
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
		nEmpty, nFull               int
		nStub, nJunk, nRefusal      int
		reasons, fullReasons, hints []string
	)
	for _, k := range keys {
		res := evalField(k, f[k])
		label := model.FieldLabels[k]
		h := res.hint
		if h == "" {
			h = rules[k].hint
		}
		switch res.st {
		case stEmpty, stStub, stJunk, stRefusal:
			nEmpty++
			what := "не заполнено"
			switch res.st {
			case stStub:
				nStub++
				what = "заглушка вместо ответа"
			case stJunk:
				nJunk++
				what = "нет сведений"
			case stRefusal:
				nRefusal++
				what = "сведения отсутствуют (отказ)"
			}
			if len(keys) > 1 {
				reasons = append(reasons, fmt.Sprintf("«%s»: %s", label, what))
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
			fullReasons = append(fullReasons, res.reason)
		}
	}
	hint = upperFirst(strings.Join(hints, "; "))
	switch {
	case nFull == len(keys):
		return w.Weight, "Заполнено полностью: " + strings.Join(fullReasons, "; "), ""
	case nEmpty == len(keys) && nRefusal > 0:
		return 0, reasonRefusal, hint
	case nEmpty == len(keys) && nJunk > 0:
		return 0, reasonJunk, hint
	case nEmpty == len(keys) && nStub > 0:
		return 0, reasonStub, hint
	case nEmpty == len(keys):
		return 0, reasonEmpty, hint
	default:
		return w.Weight / 2, "Заполнено частично: " + strings.Join(reasons, "; "), hint
	}
}

func evalField(k model.FieldKey, raw string) fieldResult {
	text := strings.Join(strings.Fields(raw), " ") // пробелы и переносы не влияют на оценку
	if text == "" {
		return fieldResult{st: stEmpty}
	}
	if kind := contactKind(text); k == model.FieldContact && kind != "" {
		return fieldResult{st: stFull, reason: "указан " + kind} // телефон состоит из цифр — проверяем до правила «без букв»
	}
	if isStub(k, text) {
		return fieldResult{st: stStub}
	}
	if isRefusal(text) {
		return fieldResult{st: stRefusal}
	}
	words, stubbed := meaningfulWords(text)
	if stubbed && words < minMeaningful {
		return fieldResult{st: stJunk} // «x клиент», «todo сервис»: без слов-заглушек смысла не остаётся
	}
	norm := normalize(text)
	if words < minMeaningful && !hasSign(k, norm) {
		return fieldResult{st: stJunk} // «asdf», «сделать хорошо», «Иван»
	}
	switch k {
	case model.FieldContext:
		if uniqueRunes(norm) < 40 {
			return fieldResult{st: stPartial, reason: "опишите, что происходит сейчас и что нужно изменить"}
		}
		return fieldResult{st: stFull, reason: "описана текущая ситуация"}
	case model.FieldNeed:
		if uniqueRunes(norm) < 40 {
			return fieldResult{st: stPartial, reason: "опишите, что происходит сейчас и что нужно изменить"}
		}
		return fieldResult{st: stFull, reason: "описана потребность"}
	case model.FieldUsers:
		if !hasAny(norm, Keywords[k]) {
			return fieldResult{st: stPartial, reason: "не указана роль пользователей"}
		}
		return fieldResult{st: stFull, reason: "названа роль (" + foundWord(norm, Keywords[k]) + ")"}
	case model.FieldExpectedResult:
		if !hasAny(norm, Keywords[k]) {
			return fieldResult{st: stPartial, reason: "нет конкретного артефакта"}
		}
		if !hasAny(norm, resultFunction) && len(uniqueWords(norm)) < 6 {
			return fieldResult{st: stPartial, reason: "назван артефакт, но не сказано, что он делает",
				hint: "допишите, что делает артефакт и для кого (например, «отчёт по продажам с фильтром по датам»)"}
		}
		return fieldResult{st: stFull, reason: "назван артефакт (" + foundWord(norm, Keywords[k]) + ") и что он делает"}
	case model.FieldSuccessCriteria:
		num, unit := measuredNumber(norm)
		if num == "" && !hasAny(norm, successVerify) {
			return fieldResult{st: stPartial, reason: "нет измеримого признака или способа проверки"}
		}
		if len(uniqueWords(norm)) < 3 {
			return fieldResult{st: stPartial, reason: "не сказано, что именно измеряем или проверяем"}
		}
		var why string
		switch {
		case num != "" && unit:
			why = "есть число с единицей (" + num + ")"
		case num != "":
			why = "есть целевое значение (" + num + ")"
		default:
			why = "описан способ проверки"
		}
		if num != "" && !hasAny(norm, successConcrete) {
			for _, a := range successAbstract {
				if strings.Contains(norm, a[0]) {
					why += "; измеримо, но не сказано, как измеряется " + a[1]
					break
				}
			}
		}
		return fieldResult{st: stFull, reason: why}
	case model.FieldData:
		return evalData(norm)
	case model.FieldConstraints:
		if !hasAny(norm, Keywords[k]) {
			return fieldResult{st: stPartial, reason: "не указаны сроки, бюджет, технологии или доступы"}
		}
		if len(uniqueWords(norm)) < 3 {
			return fieldResult{st: stPartial, reason: "не ясно, что именно ограничено"}
		}
		return fieldResult{st: stFull, reason: "указано ограничение (" + foundWord(norm, Keywords[k]) + ")"}
	case model.FieldContact:
		return fieldResult{st: stPartial, reason: "нет пригодного email, телефона или Telegram"}
	case model.FieldInteractionFormat:
		ch, rh := hasAny(norm, Keywords[k]), hasAny(norm, formatRhythm)
		switch {
		case ch && rh:
			return fieldResult{st: stFull, reason: "указаны канал и ритм общения"}
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

// hasSign — есть ли в тексте хотя бы один ключевой признак показателя поля.
// Для контекста и потребности признак — только длина, поэтому false.
func hasSign(k model.FieldKey, norm string) bool {
	switch k {
	case model.FieldSuccessCriteria:
		num, _ := measuredNumber(norm)
		return num != "" || hasAny(norm, successVerify) || hasAny(norm, Keywords[k])
	case model.FieldData:
		return hasAny(norm, Keywords[k]) || hasAny(norm, dataTransfer) || hasAny(norm, dataTransferVerb) ||
			hasAny(norm, dataPlan) || hasAny(norm, dataNegative)
	case model.FieldInteractionFormat:
		return hasAny(norm, Keywords[k]) || hasAny(norm, formatRhythm)
	case model.FieldContext, model.FieldNeed:
		return false
	}
	return hasAny(norm, Keywords[k])
}

// letterWords — буквенные слова текста в нижнем регистре, ё→е.
func letterWords(text string) []string {
	text = strings.ReplaceAll(strings.ToLower(text), "ё", "е")
	return strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) })
}

// meaningfulWords — число уникальных буквенных слов длиной ≥ 3 без слов-заглушек
// и признак того, что заглушки в тексте были.
func meaningfulWords(text string) (n int, stubbed bool) {
	ws := letterWords(text)
	seen := map[string]bool{}
	for i := 0; i < len(ws); i++ {
		if i+1 < len(ws) && stubPairs[ws[i]+" "+ws[i+1]] {
			stubbed = true
			i++
			continue
		}
		if stubWords[ws[i]] {
			stubbed = true
			continue
		}
		if utf8.RuneCountInString(ws[i]) >= 3 && !seen[ws[i]] {
			seen[ws[i]] = true
			n++
		}
	}
	return n, stubbed
}

// isRefusal — в тексте есть маркер отказа («нет», «не будет», «никаких»…) и нет плана
// или альтернативы («но», «после», «доступ», «обезличен», «соберём»…). Альтернативы
// ищутся после удаления отрицаний «не X» / «без X», чтобы «не передадим» не сошло за план.
func isRefusal(text string) bool {
	var b strings.Builder
	b.WriteByte(' ')
	for _, r := range strings.ToLower(text) {
		switch {
		case r == 'ё':
			b.WriteRune('е')
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	b.WriteByte(' ')
	line := " " + strings.Join(strings.Fields(b.String()), " ") + " "
	if !hasAny(line, refusalMarkers) {
		return false
	}
	return !hasAny(reNegated.ReplaceAllString(line, " ")+" ", refusalAlt)
}

// foundWord — слово текста, в котором найден самый ранний ключ; для коротких слов
// («до») добавляются два следующих слова («до 1 декабря»).
func foundWord(norm string, kws []string) string {
	best := -1
	for _, kw := range kws {
		if i := strings.Index(norm, kw); i >= 0 {
			i += len(kw) - len(strings.TrimLeft(kw, " "))
			if best < 0 || i < best {
				best = i
			}
		}
	}
	if best < 0 {
		return ""
	}
	start := strings.LastIndexByte(norm[:best], ' ') + 1
	rest := strings.Fields(norm[start:])
	n := 1
	if utf8.RuneCountInString(rest[0]) < 3 {
		n = min(3, len(rest))
	}
	return strings.Trim(strings.Join(rest[:n], " "), ".,+-/")
}

// transferPlanned — указан конкретный способ или срок передачи материалов.
func transferPlanned(norm string) bool {
	return hasAny(norm, dataTransfer) || hasAny(norm, dataTransferVerb) && hasAny(norm, dataTransferConcrete)
}

func evalData(norm string) fieldResult {
	material := hasAny(norm, Keywords[model.FieldData])
	if hasAny(norm, dataNegative) && !hasAny(norm, dataPlan) {
		return fieldResult{st: stPartial, reason: "материалов пока нет и не указан план их получения",
			hint: "укажите, как и когда получим материалы: кто даст доступ, что соберём (выгрузка, образцы, интервью)"}
	}
	switch {
	case material && transferPlanned(norm):
		return fieldResult{st: stFull, reason: "назван материал (" + foundWord(norm, Keywords[model.FieldData]) + ") и способ или срок передачи"}
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

// contactKind — какой пригодный контакт найден: «email», «Telegram», «телефон» или "".
// Телефон — 10–15 цифр, среди которых не меньше 3 разных («0000000000» не контакт).
func contactKind(text string) string {
	switch {
	case reEmail.MatchString(text):
		return "email"
	case reTelegram.MatchString(text):
		return "Telegram"
	}
	for _, m := range rePhone.FindAllString(text, -1) {
		n, distinct, run, maxRun := 0, map[rune]bool{}, 0, 0
		var prev rune
		for _, r := range m {
			if unicode.IsDigit(r) {
				n++
				distinct[r] = true
				if r == prev {
					run++
				} else {
					run = 1
				}
				if run > maxRun {
					maxRun = run
				}
				prev = r
			}
		}
		// ≥4 разных цифр и без серии из 5+ одинаковых: «+7 000 000 00 01», «1111111112» — не контакт
		if n >= 10 && n <= 15 && len(distinct) >= 4 && maxRun < 5 {
			return "телефон"
		}
	}
	return ""
}

// measuredNumber: есть число (не номер версии), рядом с которым стоит единица/метрика
// («30%», «10 минут», «50 заявок»), или перед которым стоит сравнение («не менее 85»,
// «до 10») либо «на/в/с N» при глаголе изменения («снизить … на 30»).
// Возвращает найденный фрагмент и unit=true, если признак — единица, а не сравнение.
func measuredNumber(norm string) (frag string, unit bool) {
	norm = reVersion.ReplaceAllString(norm, " ")
	words := strings.Fields(norm)
	change := hasAny(norm, successChange)
	kws := Keywords[model.FieldSuccessCriteria]
	for i, w := range words {
		if strings.IndexFunc(w, unicode.IsDigit) < 0 {
			continue
		}
		tail := strings.TrimLeftFunc(w, func(r rune) bool { return unicode.IsDigit(r) || strings.ContainsRune(".,+-/", r) })
		window := " " + tail + " "
		if hasAny(window, kws) {
			return w, true
		}
		for j := i + 1; j <= i+2 && j < len(words); j++ {
			window += words[j] + " "
			if hasAny(window, kws) {
				return strings.Join(words[i:j+1], " "), true
			}
		}
		for j := i - 1; j >= 0 && j >= i-2; j-- {
			if contains(successComparators, words[j]) {
				return strings.Join(words[j:i+1], " "), false
			}
			if change && j == i-1 && (words[j] == "на" || words[j] == "в" || words[j] == "с") {
				return strings.Join(words[j:i+1], " "), false
			}
		}
	}
	return "", false
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
- нет ни одного признака показателя (см. ниже) и меньше 3 значимых слов (буквенных, от 3 букв, без повторов): «asdf», «сделать хорошо», «Иван» → 0; для контекста и потребности — просто меньше 3 значимых слов → 0;
- после удаления слов-заглушек (x, todo, tbd, ok, да, тест, «не знаю», «пока нет», позже, заполню) остаётся меньше 3 значимых слов: «x клиент», «todo сервис» → 0;
- явный отказ («нет», «не будет», «никаких», «не передадим», «отсутствует»…) без плана или альтернативы («но», «вместо», «после», «доступ», «обезличен», «соберём»…) → 0;
- заполнено, но не хватает смысла (см. ниже) → 50%;
- заполнено полно → 100% (в причине указан найденный признак).
Для составных показателей (контекст + потребность, контакт + формат взаимодействия): оба пустые → 0, оба полные → 100%, иначе → 50%.

Проверка полноты (без ML; пробелы и повторы слов не учитываются, ключевые слова — регистронезависимо):
- Контекст, Потребность — описание процесса и изменения, не короче 40 символов без повторов;
- Данные — назван материал (выгрузка, таблица, CRM, API, файлы, примеры, интервью, логи) И конкретный способ/срок передачи (доступ, после NDA, в первую неделю, «передадим файлом / через неделю»…); «передадим позже / потом» — не план; «данных нет / недоступно» без плана получения → 50%;
- Ожидаемый результат — артефакт (прототип, сервис, дашборд, отчёт, бот…) И что он делает (или развёрнутое описание);
- Критерии успеха — число вместе с метрикой/единицей («на 30%», «не более 10 минут», «50 заявок») ИЛИ описание проверки (проверим, принимает, приёмка, пилот, тест, сценарий, пример, «если… / когда…»); номер версии числом не считается; от 3 разных слов; при абстрактной метрике (качество, эффективность, удобство) балл полный, но в причине предупреждение;
- Ограничения — сроки/дата/бюджет/технологии/доступы/«нельзя»/«только»/«до» и что именно ограничено (от 3 разных слов);
- Пользователи — названа роль (менеджер, сотрудник, клиент, врач, отдел…), длина не важна;
- Контакт — email, Telegram (@username от 5 символов, t.me/…) или телефон (10–15 цифр, из них не меньше 3 разных); иной текст → 50%, «@»/«tg» → 0;
- Формат взаимодействия — канал (созвон, встреча, чат, Telegram, email…) И ритм или срок ответа (раз в неделю, еженедельно, в течение дня…); одно из двух → 50%.

Уровни: 0–39 — черновик, 40–69 — рабочая, 70–89 — готовая, 90–100 — приоритетная.
Подтверждение карточки не меняет формулу: оно публикует задачу; до подтверждения балл предварительный.
`)
	return b.String()
}
