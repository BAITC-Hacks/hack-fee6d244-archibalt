package ai

import (
	"fmt"
	"strings"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// PromptQuestions — системный промпт для уточняющих вопросов (показывается на /api/ai).
const PromptQuestions = `Ты помощник, который проверяет полноту описания бизнес-задачи для студенческих команд. ` +
	`Карточка задачи состоит из полей: title — Название; context — Контекст; need — Потребность; users — Пользователи; ` +
	`data — Данные и материалы; constraints — Ограничения; expected_result — Ожидаемый результат; ` +
	`success_criteria — Критерии успеха; contact — Контакт; interaction_format — Формат взаимодействия. ` +
	`По тексту черновика определи, какие поля не раскрыты, и задай НЕ МЕНЕЕ ТРЁХ и не более четырёх уточняющих вопросов, ` +
	`каждый строго про одно недостающее поле. Не переспрашивай то, что уже сказано; если контекст и потребность понятны — не спрашивай о них. Цена вопроса высока: спрашивай только то, без чего команда не сможет начать. Порядок и тон: бизнес обычно знает свою боль, а не продукт, ` +
	`поэтому первый вопрос — про то, что сейчас происходит и что от этого теряется (context/need), ` +
	`затем — какие данные и материалы есть, как поймём результат, кто пользователи, сроки и ограничения, контакт. ` +
	`Вопрос — одно короткое предложение простыми словами, без терминов, с примером ответа в скобках, если уместно. ` +
	`Не предлагай решений и не придумывай фактов. Отвечай JSON по схеме.`

// PromptNextQuestion — системный промпт пошагового режима: один следующий вопрос или done.
const PromptNextQuestion = `Ты помощник, который проверяет полноту описания бизнес-задачи для студенческих команд. ` +
	`Карточка задачи состоит из полей: title — Название; context — Контекст; need — Потребность; users — Пользователи; ` +
	`data — Данные и материалы; constraints — Ограничения; expected_result — Ожидаемый результат; ` +
	`success_criteria — Критерии успеха; contact — Контакт; interaction_format — Формат взаимодействия. ` +
	`Вот черновик и уже заданные вопросы с ответами. Определи, каких полей карточки всё ещё не хватает, ` +
	`чтобы студенческая команда могла начать (missing_fields). ` +
	`Если задано меньше 3 вопросов или не хватает критичного (данные, критерии успеха, контакт или формат общения) — ` +
	`верни done=false и ОДИН следующий вопрос строго про одно поле: одно короткое предложение простыми словами, ` +
	`с примером ответа в скобках. Предложи 2–4 вероятных варианта ответа простыми словами (suggestions), если вопрос это позволяет; ` +
	`input_type=choice, если ответ — один из вариантов; multi, если можно выбрать несколько (варианты — процессы или сущности из черновика); ` +
	`yes_no — для подтверждения (suggestions пустой); иначе text. Не переспрашивай то, что уже известно из черновика или ответов, ` +
	`и не повторяй поля уже заданных вопросов (даже если ответ пропущен). ` +
	`Иначе верни done=true, question=null и в reason — одной фразой, почему сведений достаточно. ` +
	`Не предлагай решений и не выдумывай фактов. Отвечай JSON по схеме.`

// PromptCard — системный промпт для сборки карточки (показывается на /api/ai).
const PromptCard = `Собери карточку задачи ТОЛЬКО из сведений, которые сообщил пользователь в черновике и ответах. ` +
	`Если сведений для поля нет — верни пустую строку. Не добавляй фактов, не домысливай цифры, имена, сроки. ` +
	`Формулировки можно сжать, но смысл — только из текста пользователя. ` +
	`Значение поля — строго по его смыслу, не перефраз черновика: ` +
	`title — короткое название задачи; context — что происходит сейчас (как устроен процесс, масштаб) — сохраняй формулировки пользователя почти дословно; need — что нужно изменить или получить — тоже словами пользователя, не переформулируй проблему в цель; ` +
	`users — для кого решение (роли, группы); data — какие данные, примеры или материалы будут переданы и когда; ` +
	`constraints — ТОЛЬКО явные границы: сроки, бюджет, технологии, доступы, запреты (если пользователь их не называл — пустая строка); ` +
	`expected_result — конкретный артефакт работы команды; success_criteria — измеримые признаки приёмки; ` +
	`contact — как связаться (email, телефон, Telegram); interaction_format — канал и ритм общения. ` +
	`Одно и то же предложение не копируй в несколько полей. Отвечай JSON по схеме.`

// SchemaExample — формат входа и выхода обеих функций (для /api/ai).
const SchemaExample = `// 1) Уточняющие вопросы
// вход:  {"industry": "Логистика", "draft": "Нужно приложение для учёта заявок"}
// выход: {"questions": [{"text": "Какие данные, примеры или источники вы сможете передать команде?", "field_key": "data"}, ...≥3],
//         "missing_fields": ["context", "data", "success_criteria"]}
// 2) Карточка
// вход:  черновик + [{"text": "...", "field_key": "data", "answer": "Выгрузка заявок из Excel за год"}]
// выход: {"fields": {"title": "...", "context": "...", "need": "...", "users": "", "data": "Выгрузка заявок из Excel за год",
//         "constraints": "", "expected_result": "", "success_criteria": "", "contact": "", "interaction_format": ""}}
// Обработка некорректного ответа: статус ≠ 200, невалидный JSON, неизвестная структура или < 3 вопросов →
// один повтор, затем локальная заглушка (mode = "mock", last_error = текст ошибки).
// Защита от выдумок: каждое непустое поле (кроме title) проверяется — ≥ 60% его значимых слов и все числа
// должны встречаться в черновике или ответах пользователя, иначе поле очищается.`

func fieldKeyStrings() []string {
	out := make([]string, len(model.FieldKeys))
	for i, k := range model.FieldKeys {
		out[i] = string(k)
	}
	return out
}

func inputTypeStrings() []string {
	out := make([]string, len(model.InputTypes))
	for i, t := range model.InputTypes {
		out[i] = string(t)
	}
	return out
}

func questionsSchema() map[string]any {
	enum := map[string]any{"type": "string", "enum": fieldKeyStrings()}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"questions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"text":      map[string]any{"type": "string"},
						"field_key": enum,
					},
					"required":             []string{"text", "field_key"},
					"additionalProperties": false,
				},
			},
			"missing_fields": map[string]any{"type": "array", "items": enum},
		},
		"required":             []string{"questions", "missing_fields"},
		"additionalProperties": false,
	}
}

func nextQuestionSchema() map[string]any {
	enum := map[string]any{"type": "string", "enum": fieldKeyStrings()}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"done":   map[string]any{"type": "boolean"},
			"reason": map[string]any{"type": "string"},
			"question": map[string]any{"anyOf": []any{
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"text":        map[string]any{"type": "string"},
						"field_key":   enum,
						"input_type":  map[string]any{"type": "string", "enum": inputTypeStrings()},
						"suggestions": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
					"required":             []string{"text", "field_key", "input_type", "suggestions"},
					"additionalProperties": false,
				},
				map[string]any{"type": "null"},
			}},
			"missing_fields": map[string]any{"type": "array", "items": enum},
		},
		"required":             []string{"done", "reason", "question", "missing_fields"},
		"additionalProperties": false,
	}
}

func cardSchema() map[string]any {
	props := map[string]any{}
	for _, k := range model.FieldKeys {
		props[string(k)] = map[string]any{"type": "string"}
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fields": map[string]any{
				"type":                 "object",
				"properties":           props,
				"required":             fieldKeyStrings(),
				"additionalProperties": false,
			},
		},
		"required":             []string{"fields"},
		"additionalProperties": false,
	}
}

func questionsUserMessage(draft, industry string) string {
	return fmt.Sprintf("Отрасль: %s\nЧерновик:\n%s", industry, draft)
}

func cardUserMessage(draft, industry string, qs []model.Question) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Отрасль: %s\nЧерновик:\n%s\n\nОтветы на уточняющие вопросы:\n", industry, draft)
	for _, q := range qs {
		if strings.TrimSpace(q.Answer) == "" {
			continue
		}
		fmt.Fprintf(&b, "Вопрос: %s / Ответ: %s\n", q.Text, q.Answer)
	}
	return b.String()
}

func nextQuestionUserMessage(draft, industry string, asked []model.Question) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Отрасль: %s\nЧерновик:\n%s\n\n", industry, draft)
	if len(asked) == 0 {
		b.WriteString("Вопросов ещё не задавали.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "Уже заданные вопросы (%d):\n", len(asked))
	for _, q := range asked {
		a := strings.TrimSpace(q.Answer)
		if a == "" {
			a = "(пропущен)"
		}
		fmt.Fprintf(&b, "- [%s] Вопрос: %s / Ответ: %s\n", q.FieldKey, q.Text, a)
	}
	return b.String()
}

// PromptResultOptions — системный промпт вариантов первого проверяемого результата (показывается на /api/ai).
const PromptResultOptions = `Ты помогаешь бизнесу выбрать первый посильный результат для студенческой команды. ` +
	`По черновику и ответам предложи 2–3 РАЗНЫХ по объёму варианта первого результата: от малого к среднему. ` +
	`Каждый вариант проверяемый: бизнес сам может убедиться, что результат готов. Поля варианта: ` +
	`title — название из 3–6 слов; result — что именно команда передаст бизнесу, 1–2 предложения; ` +
	`check — как бизнес проверит готовность: наблюдаемое действие или измеримый признак, без выдуманных цифр; ` +
	`needs — что нужно от бизнеса: данные, доступы, время людей; если чего-то пользователь не называл — так и напиши, что это нужно уточнить; ` +
	`weeks — оценка срока в неделях, целое от 2 до 8. ` +
	`Ссылайся только на сущности, данные и процессы, которые назвал пользователь. Не придумывай цифр, систем, программ, названий компаний и людей; ` +
	`не утверждай, что бизнес уже дал доступ или согласовал сроки. Срок пиши только в weeks, не в тексте. ` +
	`Пиши простыми словами, без терминов. Отвечай JSON по схеме.`

func resultOptionsSchema() map[string]any {
	str := map[string]any{"type": "string"}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"options": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title": str, "result": str, "check": str, "needs": str,
						"weeks": map[string]any{"type": "integer"},
					},
					"required":             []string{"title", "result", "check", "needs", "weeks"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"options"},
		"additionalProperties": false,
	}
}
