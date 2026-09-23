package ai

import (
	"context"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/rating"
)

// mockClient — детерминированная заглушка без сети.
type mockClient struct{}

// priority — порядок важности полей для вопросов (по весам рейтинга).
var priority = []model.FieldKey{
	model.FieldContext, model.FieldNeed, model.FieldData, model.FieldExpectedResult, model.FieldSuccessCriteria,
	model.FieldConstraints, model.FieldUsers, model.FieldContact, model.FieldInteractionFormat, model.FieldTitle,
}

// padOrder — чем добивать до 3 вопросов, если недостающих полей мало.
var padOrder = append([]model.FieldKey{model.FieldTitle, model.FieldInteractionFormat}, priority...)

// keywords — признаки того, что поле раскрыто в черновике (подстроки, нижний регистр).
var keywords = map[model.FieldKey][]string{
	model.FieldUsers:             {"пользовател", "клиент", "сотрудник", "студент"},
	model.FieldData:              {"данн", "таблиц", "файл", "выгруз", "crm", "api"},
	model.FieldConstraints:       {"срок", "бюджет", "до ", "нельзя", "технолог"},
	model.FieldExpectedResult:    {"результат", "прототип", "отчёт", "отчет", "сервис", "приложен"},
	model.FieldSuccessCriteria:   {"%", "критери", "метрик", "kpi", "снижен", "рост"},
	model.FieldContact:           {"@", "+7", "телефон", "почт"},
	model.FieldInteractionFormat: {"встреч", "созвон", "раз в", "чат"},
	model.FieldNeed:              {"нужно", "хотим", "требуется", "проблем"},
}

// questionTexts — готовая формулировка вопроса на каждое поле.
var questionTexts = map[model.FieldKey]string{
	model.FieldTitle:             "Как коротко назвать задачу, чтобы студентам было понятно, о чём она?",
	model.FieldContext:           "Чем занимается ваша компания или отдел и в какой ситуации возникла задача?",
	model.FieldNeed:              "Какую проблему нужно решить и что сейчас работает не так?",
	model.FieldUsers:             "Кто будет пользоваться результатом: клиенты, сотрудники, какой отдел?",
	model.FieldData:              "Какие данные, примеры или источники вы сможете передать команде?",
	model.FieldConstraints:       "Какие есть ограничения: сроки, бюджет, обязательные или запрещённые технологии?",
	model.FieldExpectedResult:    "Что вы хотите получить в итоге: прототип, сервис, отчёт или что-то другое?",
	model.FieldSuccessCriteria:   "По каким измеримым признакам вы поймёте, что задача решена успешно?",
	model.FieldContact:           "Кто будет контактным лицом для команды и как с ним связаться?",
	model.FieldInteractionFormat: "Как часто и в каком формате вы готовы общаться с командой: встречи, созвоны, чат?",
}

// missingFields — эвристика: какие поля не раскрыты в черновике (в порядке priority).
func missingFields(draft string) []model.FieldKey {
	low := strings.ToLower(draft)
	var out []model.FieldKey
	for _, k := range priority {
		closed := true
		switch k {
		case model.FieldTitle:
			// название формируется из черновика, отдельно не проверяется
		case model.FieldContext:
			closed = utf8.RuneCountInString(strings.TrimSpace(draft)) >= 80
		default:
			closed = false
			for _, w := range keywords[k] {
				if strings.Contains(low, w) {
					closed = true
					break
				}
			}
		}
		if !closed {
			out = append(out, k)
		}
	}
	return out
}

// fillQuestions дополняет qs готовыми вопросами по полям из prefer, затем padOrder,
// пропуская уже покрытые поля, пока вопросов меньше n. ID проставляются 1..len.
func fillQuestions(qs []model.Question, prefer []model.FieldKey, n int) []model.Question {
	used := map[model.FieldKey]bool{}
	for _, q := range qs {
		used[q.FieldKey] = true
	}
	for _, list := range [][]model.FieldKey{prefer, padOrder} {
		for _, k := range list {
			if len(qs) >= n {
				break
			}
			if used[k] {
				continue
			}
			used[k] = true
			qs = append(qs, model.Question{Text: questionTexts[k], FieldKey: k})
		}
	}
	for i := range qs {
		qs[i].ID = i + 1
	}
	return qs
}

func (mockClient) questions(_ context.Context, draft, _ string) (model.QuestionsResult, error) {
	missing := missingFields(draft)
	n := min(5, len(missing))
	qs := fillQuestions(nil, missing[:n], max(3, n))
	if missing == nil {
		missing = []model.FieldKey{}
	}
	return model.QuestionsResult{Questions: qs, MissingFields: missing}, nil
}

// fieldInputs — тип ответа и подсказки-варианты на каждое поле для пошагового режима (mock и дополнение).
var fieldInputs = map[model.FieldKey]struct {
	typ         model.InputType
	suggestions []string
}{
	model.FieldTitle:             {model.InputText, nil},
	model.FieldContext:           {model.InputText, []string{"Небольшая компания, всё ведём в таблицах", "Отдел внутри крупной компании", "Проект только запускается"}},
	model.FieldNeed:              {model.InputText, []string{"Заявки теряются", "Много ручной работы", "Руководству не видна общая картина"}},
	model.FieldUsers:             {model.InputMulti, []string{"Клиенты", "Сотрудники", "Руководство"}},
	model.FieldData:              {model.InputChoice, []string{"Выгрузка из Excel/CRM", "Примеры документов", "Данных нет, соберём"}},
	model.FieldConstraints:       {model.InputMulti, []string{"Жёсткий срок", "Без бюджета на сервисы", "Только бесплатные инструменты", "Ограничений нет"}},
	model.FieldExpectedResult:    {model.InputChoice, []string{"Прототип", "Рабочий сервис", "Аналитический отчёт"}},
	model.FieldSuccessCriteria:   {model.InputText, []string{"Заявки не теряются", "Обработка заявки быстрее вдвое", "Руководитель видит отчёт без ручной сводки"}},
	model.FieldContact:           {model.InputText, nil},
	model.FieldInteractionFormat: {model.InputChoice, []string{"Созвон раз в неделю", "Чат в Telegram", "Встреча раз в две недели"}},
}

// stockQuestion — готовый вопрос пошагового режима по полю: текст, тип ответа, подсказки.
func stockQuestion(k model.FieldKey) *model.Question {
	in := fieldInputs[k]
	return &model.Question{Text: questionTexts[k], FieldKey: k, InputType: in.typ, Suggestions: slices.Clone(in.suggestions)}
}

// cleanSuggestions: непустые, без повторов, не больше 4; у yes_no подсказок нет.
func cleanSuggestions(typ model.InputType, in []string) []string {
	if typ == model.InputYesNo {
		return nil
	}
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" && !slices.Contains(out, s) && len(out) < 4 {
			out = append(out, s)
		}
	}
	return out
}

// criticalFields — без них команда не начнёт: пошаговый режим не завершается сам, пока они не закрыты
// или не переспрошены maxAsksPerField раз.
var criticalFields = []model.FieldKey{model.FieldData, model.FieldExpectedResult, model.FieldSuccessCriteria, model.FieldContact}

// retryTexts — вторая формулировка вопроса, если на первую ответили без сведений («здравствуйте», «не знаю», пропуск).
var retryTexts = map[model.FieldKey]string{
	model.FieldTitle:             "Давайте иначе: как бы вы назвали эту задачу в одном сообщении коллеге?",
	model.FieldContext:           "Давайте иначе: вспомните последний случай, когда это мешало работе, — что тогда произошло?",
	model.FieldNeed:              "Давайте иначе: что вас больше всего раздражает в текущем процессе (например, потерянные заявки или ручная сводка)?",
	model.FieldUsers:             "Давайте иначе: кто первым заметит, что стало лучше, — клиенты, сотрудники или руководство?",
	model.FieldData:              "Давайте иначе: где сейчас хранится эта информация — Excel, CRM, бумага, мессенджер? Можно выбрать вариант.",
	model.FieldConstraints:       "Давайте иначе: есть ли срок или сумма, которые нельзя превысить? Можно ответить «ограничений нет».",
	model.FieldExpectedResult:    "Давайте иначе: что команда должна показать вам в конце — прототип, работающий сервис или отчёт?",
	model.FieldSuccessCriteria:   "Давайте иначе: что должно измениться через месяц после запуска, чтобы вы сказали «получилось»?",
	model.FieldContact:           "Давайте иначе: кому команда может написать с вопросами — email, телефон или Telegram?",
	model.FieldInteractionFormat: "Давайте иначе: как вам удобнее — короткий созвон раз в неделю или переписка в чате?",
}

// questionCap — потолок числа вопросов: 5, по явному запросу «ещё» — 8.
func questionCap(more bool) int {
	if more {
		return MaxExtraQuestions
	}
	return MaxDynamicQuestions
}

// fieldState — closed: на поле дан ответ со сведениями (не шум и не пропуск); count — сколько раз поле спрошено.
func fieldState(asked []model.Question) (closed map[model.FieldKey]bool, count map[model.FieldKey]int) {
	closed, count = map[model.FieldKey]bool{}, map[model.FieldKey]int{}
	for _, q := range asked {
		count[q.FieldKey]++
		if !rating.IsNoise(q.FieldKey, q.Answer) {
			closed[q.FieldKey] = true
		}
	}
	return closed, count
}

// askable — поле можно спросить: не закрыто и спрошено меньше maxAsksPerField раз (без зацикливания).
func askable(k model.FieldKey, closed map[model.FieldKey]bool, count map[model.FieldKey]int) bool {
	return !closed[k] && count[k] < maxAsksPerField
}

// stockQuestionFor — готовый вопрос по полю; повторный — другой формулировкой.
func stockQuestionFor(k model.FieldKey, count map[model.FieldKey]int) *model.Question {
	q := stockQuestion(k)
	if count[k] > 0 && retryTexts[k] != "" {
		q.Text = retryTexts[k]
	}
	return q
}

// nextQuestion (mock): поля, не раскрытые ни в черновике, ни в ответах со сведениями; до MinDynamicQuestions
// (или по more) — по приоритету, дальше — только критичные, иначе done. Поле с ответом-шумом переспрашивается.
func (mockClient) nextQuestion(_ context.Context, draft, _ string, asked []model.Question, more bool) (model.NextQuestionResult, error) {
	text := draft
	for _, q := range asked {
		if !rating.IsNoise(q.FieldKey, q.Answer) {
			text += "\n" + q.Answer
		}
	}
	closed, count := fieldState(asked)
	r := model.NextQuestionResult{MissingFields: []model.FieldKey{}}
	for _, k := range missingFields(text) {
		if !closed[k] {
			r.MissingFields = append(r.MissingFields, k)
		}
	}
	var candidates []model.FieldKey
	for _, k := range r.MissingFields {
		if askable(k, closed, count) && (len(asked) < MinDynamicQuestions || more || slices.Contains(criticalFields, k)) {
			candidates = append(candidates, k)
		}
	}
	if len(candidates) == 0 && len(asked) >= MinDynamicQuestions && !more {
		r.Done, r.Reason = true, "ключевые сведения есть или уже уточнены: данные, ожидаемый результат, критерии успеха, контакт"
		return r, nil
	}
	if len(candidates) > 0 {
		r.Question = stockQuestionFor(candidates[0], count)
	}
	return r, nil // пусто при < Min или more — finishNext добьёт по padOrder
}

// finishNext — правила пошагового режима поверх любого backend: валидный ключ; поле, закрытое ответом
// со сведениями, не спрашивается, а после шума/пропуска — переспрашивается (не больше maxAsksPerField раз);
// < MinDynamicQuestions — всегда вопрос (done игнорируется); ≥ MaxDynamicQuestions — done;
// more (явный запрос «ещё») — done модели игнорируется, потолок MaxExtraQuestions.
func finishNext(r model.NextQuestionResult, asked []model.Question, draft string, more bool) model.NextQuestionResult {
	closed, count := fieldState(asked)
	// Название не спрашиваем никогда (берётся из черновика); контекст и потребность — только если черновик
	// совсем короткий: «не переспрашивай известное», цена вопроса высока.
	closed[model.FieldTitle] = true
	if len([]rune(strings.TrimSpace(draft))) >= 60 {
		closed[model.FieldContext] = true
		closed[model.FieldNeed] = true
	}
	missing := []model.FieldKey{}
	for _, k := range r.MissingFields {
		if validKey(k) && !closed[k] && !slices.Contains(missing, k) {
			missing = append(missing, k)
		}
	}
	r.MissingFields = missing
	n := len(asked)
	if n >= questionCap(more) {
		r.Question, r.Done = nil, true
		if more {
			r.Reason = "задано максимальное число вопросов (8), соберите карточку"
		} else if r.Reason == "" {
			r.Reason = "задано максимальное число вопросов"
		}
		return r
	}
	if q := r.Question; q != nil {
		r.Question = nil
		if t := strings.TrimSpace(q.Text); t != "" && validKey(q.FieldKey) && askable(q.FieldKey, closed, count) {
			typ := q.InputType
			if !slices.Contains(model.InputTypes, typ) {
				typ = model.InputText
			}
			sg := cleanSuggestions(typ, q.Suggestions)
			if (typ == model.InputChoice || typ == model.InputMulti) && len(sg) < 2 {
				typ = model.InputText // выбирать не из чего — свободный ответ
			}
			for _, old := range asked {
				if old.FieldKey == q.FieldKey && strings.TrimSpace(old.Text) == t && retryTexts[q.FieldKey] != "" {
					t = retryTexts[q.FieldKey] // модель повторила вопрос дословно — другая формулировка
				}
			}
			r.Question = &model.Question{Text: t, FieldKey: q.FieldKey, InputType: typ, Suggestions: sg}
		}
	}
	if r.Done && n >= MinDynamicQuestions && !more {
		r.Question = nil
		return r
	}
	if r.Question == nil {
		lists := [][]model.FieldKey{missing}
		if n < MinDynamicQuestions || more {
			lists = append(lists, padOrder)
		}
	pick:
		for _, list := range lists {
			for _, k := range list {
				if askable(k, closed, count) {
					r.Question = stockQuestionFor(k, count)
					break pick
				}
			}
		}
	}
	if r.Question == nil { // n ≥ Min, модель не дала годного вопроса и спрашивать больше нечего
		r.Done = true
		if r.Reason == "" {
			r.Reason = "недостающие поля уже спрошены"
		}
		return r
	}
	r.Done, r.Reason = false, ""
	return r
}

func (mockClient) card(_ context.Context, draft, industry string, qs []model.Question) (model.CardResult, error) {
	f := model.Fields{model.FieldTitle: firstWords(draft, 8)}
	answered := map[model.FieldKey]bool{}
	for _, q := range qs {
		a := strings.TrimSpace(q.Answer)
		if a == "" || !validKey(q.FieldKey) {
			continue
		}
		if answered[q.FieldKey] {
			f[q.FieldKey] += " " + a
		} else {
			f[q.FieldKey] = a
		}
		answered[q.FieldKey] = true
	}
	if !answered[model.FieldContext] {
		f[model.FieldContext] = strings.TrimSpace(draft)
	}
	if !answered[model.FieldNeed] {
		if need := needFragment(draft); need != "" {
			f[model.FieldNeed] = need
		}
	}
	return finishCard(f, draft, industry, qs), nil
}

// needMarkers — слова, с которых в черновике начинается формулировка потребности.
var needMarkers = map[string]bool{"нужно": true, "хотим": true, "требуется": true, "надо": true, "необходимо": true}

// needFragment — первый фрагмент черновика от маркера потребности (включительно) до конца
// предложения: «…, поэтому нужно приложение для учёта.» → «Нужно приложение для учёта».
// Маркер сохраняется, чтобы поле читалось как законченная фраза. Без маркера — "".
func needFragment(draft string) string {
	sentences := strings.FieldsFunc(draft, func(r rune) bool { return strings.ContainsRune(".!?;\n", r) })
	for _, sent := range sentences {
		ws := strings.Fields(sent)
		for i := 0; i+1 < len(ws); i++ {
			if !needMarkers[strings.ToLower(strings.Trim(ws[i], ",:—–-«»\"()"))] {
				continue
			}
			frag := strings.TrimRight(strings.Join(ws[i:], " "), ",:;—–- ")
			r, n := utf8.DecodeRuneInString(frag)
			return string(unicode.ToUpper(r)) + frag[n:]
		}
	}
	return ""
}

func validKey(k model.FieldKey) bool {
	_, ok := model.FieldLabels[k]
	return ok
}
