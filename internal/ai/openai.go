package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

const defaultModel = "gpt-6-sol"

// openAIClient — вызов OpenAI Responses API через net/http, без SDK.
type openAIClient struct {
	apiKey    string
	model     string
	fastModel string
	endpoint  string
	http      *http.Client
}

func newOpenAI(apiKey, modelName, endpoint string) *openAIClient {
	if modelName == "" {
		modelName = defaultModel
	}
	fast := os.Getenv("OPENAI_MODEL_FAST") // для вопросов: быстрая модель (2–3 с), карточка — основная
	if fast == "" {
		fast = modelName
	}
	return &openAIClient{apiKey: apiKey, model: modelName, fastModel: fast, endpoint: endpoint, http: &http.Client{Timeout: 10 * time.Second}}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responsesRequest struct {
	Model string    `json:"model"`
	Input []message `json:"input"`
	Text  struct {
		Format struct {
			Type   string         `json:"type"`
			Name   string         `json:"name"`
			Strict bool           `json:"strict"`
			Schema map[string]any `json:"schema"`
		} `json:"format"`
	} `json:"text"`
}

type responsesResponse struct {
	Output []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func truncate(b []byte) string {
	r := []rune(string(b))
	if len(r) > 500 {
		r = r[:500]
	}
	return string(r)
}

// call отправляет запрос и декодирует JSON из output_text в out.
func (c *openAIClient) call(ctx context.Context, name, system, user string, schema map[string]any, out any) error {
	return c.callWith(ctx, c.model, name, system, user, schema, out)
}

func (c *openAIClient) callWith(ctx context.Context, mdl, name, system, user string, schema map[string]any, out any) error {
	var req responsesRequest
	req.Model = mdl
	req.Input = []message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	req.Text.Format.Type = "json_schema"
	req.Text.Format.Name = name
	req.Text.Format.Strict = true
	req.Text.Format.Schema = schema
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	hr.Header.Set("Authorization", "Bearer "+c.apiKey)
	hr.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(hr)
	if err != nil {
		return fmt.Errorf("openai: запрос: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("openai: чтение ответа: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("openai: статус %d: %s", resp.StatusCode, truncate(raw))
	}
	var rr responsesResponse
	if err := json.Unmarshal(raw, &rr); err != nil {
		return fmt.Errorf("openai: невалидный JSON ответа: %v: %s", err, truncate(raw))
	}
	for _, o := range rr.Output {
		for _, ct := range o.Content {
			if ct.Type != "output_text" {
				continue
			}
			if err := json.Unmarshal([]byte(ct.Text), out); err != nil {
				return fmt.Errorf("openai: невалидный JSON в output_text: %v: %s", err, truncate([]byte(ct.Text)))
			}
			return nil
		}
	}
	return fmt.Errorf("openai: в ответе нет output_text: %s", truncate(raw))
}

func (c *openAIClient) questions(ctx context.Context, draft, industry string) (model.QuestionsResult, error) {
	var res struct {
		Questions []struct {
			Text     string         `json:"text"`
			FieldKey model.FieldKey `json:"field_key"`
		} `json:"questions"`
		MissingFields []model.FieldKey `json:"missing_fields"`
	}
	if err := c.callWith(ctx, c.fastModel, "clarifying_questions", PromptQuestions, questionsUserMessage(draft, industry), questionsSchema(), &res); err != nil {
		return model.QuestionsResult{}, err
	}
	// по одному вопросу на поле, только валидные ключи, не больше 5
	var qs []model.Question
	used := map[model.FieldKey]bool{}
	for _, q := range res.Questions {
		t := strings.TrimSpace(q.Text)
		if t == "" || !validKey(q.FieldKey) || used[q.FieldKey] || len(qs) == 5 {
			continue
		}
		used[q.FieldKey] = true
		qs = append(qs, model.Question{Text: t, FieldKey: q.FieldKey})
	}
	if len(qs) == 0 {
		return model.QuestionsResult{}, errors.New("openai: нет валидных вопросов в ответе")
	}
	missing := []model.FieldKey{}
	seen := map[model.FieldKey]bool{}
	for _, k := range res.MissingFields {
		if validKey(k) && !seen[k] {
			seen[k] = true
			missing = append(missing, k)
		}
	}
	// меньше 3 — дополняем готовыми вопросами по missing_fields
	qs = fillQuestions(qs, missing, 3)
	return model.QuestionsResult{Questions: qs, MissingFields: missing}, nil
}

func (c *openAIClient) card(ctx context.Context, draft, industry string, qs []model.Question) (model.CardResult, error) {
	var res struct {
		Fields map[model.FieldKey]string `json:"fields"`
	}
	if err := c.call(ctx, "task_card", PromptCard, cardUserMessage(draft, industry, qs), cardSchema(), &res); err != nil {
		return model.CardResult{}, err
	}
	if res.Fields == nil {
		return model.CardResult{}, errors.New("openai: в ответе нет fields")
	}
	return finishCard(model.Fields(res.Fields), draft, industry, qs), nil
}

func (c *openAIClient) nextQuestion(ctx context.Context, draft, industry string, asked []model.Question, more bool) (model.NextQuestionResult, error) {
	var res struct {
		Done     bool   `json:"done"`
		Reason   string `json:"reason"`
		Question *struct {
			Text        string          `json:"text"`
			FieldKey    model.FieldKey  `json:"field_key"`
			InputType   model.InputType `json:"input_type"`
			Suggestions []string        `json:"suggestions"`
		} `json:"question"`
		MissingFields []model.FieldKey `json:"missing_fields"`
	}
	if err := c.callWith(ctx, c.fastModel, "next_question", PromptNextQuestion, nextQuestionUserMessage(draft, industry, asked, more), nextQuestionSchema(), &res); err != nil {
		return model.NextQuestionResult{}, err
	}
	r := model.NextQuestionResult{Done: res.Done, Reason: strings.TrimSpace(res.Reason), MissingFields: res.MissingFields}
	if res.Question != nil {
		q := res.Question
		r.Question = &model.Question{Text: q.Text, FieldKey: q.FieldKey, InputType: q.InputType, Suggestions: guardSuggestions(q.Suggestions, sourceText(draft, industry, asked))}
	}
	return r, nil // валидация и правила 3..5 (по more — до 8) — в finishNext
}

// guardSuggestions — подсказки-кнопки становятся ответом пользователя и обходят Guard, поэтому
// проверяются заранее: выкидываем варианты с числами, именами собственными и email, которых нет в источнике.
func guardSuggestions(in []string, source string) []string {
	src := sourceIndex(source)
	low := strings.ToLower(source)
	out := make([]string, 0, len(in))
	for _, s := range in {
		if inventedName(s, src) || inventedNumber(s, src) {
			continue
		}
		if i := strings.Index(s, "@"); i >= 0 && !strings.Contains(low, strings.ToLower(strings.Fields(s[max(0, i-30):])[0])) {
			continue
		}
		out = append(out, s)
	}
	return out
}
