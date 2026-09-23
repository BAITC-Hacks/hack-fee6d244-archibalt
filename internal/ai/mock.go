package ai

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
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
