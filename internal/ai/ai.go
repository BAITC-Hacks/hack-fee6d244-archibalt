// Package ai — AI-функции ТЗ §5: уточняющие вопросы по черновику и сборка карточки
// только из слов пользователя. Работает через OpenAI Responses API, при любой ошибке
// (или без ключа) — детерминированная локальная заглушка (mock).
package ai

import (
	"context"
	"errors"
	"sync"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// Client — контракт для HTTP-хендлеров.
type Client interface {
	Questions(ctx context.Context, draft string, industry string) (model.QuestionsResult, error)
	// Card собирает карточку; qs — вопросы с заполненным Answer.
	Card(ctx context.Context, draft string, industry string, qs []model.Question) (model.CardResult, error)
	// NextQuestion — пошаговый режим: следующий вопрос с учётом уже заданных (asked, с ответами)
	// или Done. Не меньше MinDynamicQuestions и не больше MaxDynamicQuestions вопросов; more — явный запрос
	// «спросить ещё»: done модели игнорируется, потолок поднимается до MaxExtraQuestions.
	NextQuestion(ctx context.Context, draft, industry string, asked []model.Question, more bool) (model.NextQuestionResult, error)
	// ResultOptions — 2–3 варианта первого проверяемого результата (малый → средний) по черновику и ответам qs.
	ResultOptions(ctx context.Context, draft, industry string, qs []model.Question) (model.ResultOptionsResult, error)
	// Mode — режим, фактически использованный в последнем вызове.
	Mode() model.AIMode
	// Info — данные для GET /api/ai.
	Info() Info
}

// Info — ответ GET /api/ai.
type Info struct {
	Mode            model.AIMode `json:"mode"`
	PromptQuestions string       `json:"prompt_questions"`
	PromptCard      string       `json:"prompt_card"`
	PromptNext      string       `json:"prompt_next_question"`
	PromptOptions   string       `json:"prompt_result_options"`
	SchemaExample   string       `json:"schema_example"`
	LastError       *string      `json:"last_error"`
	LastCall        *LastCall    `json:"last_call"`
}

// Границы пошагового режима: не меньше Min и не больше Max вопросов.
const (
	MinDynamicQuestions = 3
	MaxDynamicQuestions = 5
	MaxExtraQuestions   = 8 // потолок по явному запросу «спросить ещё»
	maxAsksPerField     = 2 // поле без сведений в ответе переспрашивается не больше 2 раз
)

const defaultEndpoint = "https://api.openai.com/v1/responses"

// New: apiKey == "" → только mock; иначе OpenAI с повтором и откатом на mock.
func New(apiKey, modelName string) Client {
	return newClient(apiKey, modelName, defaultEndpoint)
}

func newClient(apiKey, modelName, endpoint string) *fallback {
	f := &fallback{backup: mockClient{}, mode: model.AIModeMock}
	if apiKey != "" {
		f.primary = newOpenAI(apiKey, modelName, endpoint)
		f.mode = model.AIModeOpenAI
	}
	return f
}

// backend — внутренний интерфейс режима (openai / mock).
type backend interface {
	questions(ctx context.Context, draft, industry string) (model.QuestionsResult, error)
	card(ctx context.Context, draft, industry string, qs []model.Question) (model.CardResult, error)
	nextQuestion(ctx context.Context, draft, industry string, asked []model.Question, more bool) (model.NextQuestionResult, error)
	resultOptions(ctx context.Context, draft, industry string, qs []model.Question) (model.ResultOptionsResult, error)
}

// fallback: primary (openai, может быть nil) → один повтор → backup (mock).
type fallback struct {
	primary backend
	backup  backend

	mu      sync.Mutex
	mode    model.AIMode
	lastErr error
}

func (f *fallback) Questions(ctx context.Context, draft, industry string) (model.QuestionsResult, error) {
	return run(f, func(b backend) (model.QuestionsResult, error) {
		r, err := b.questions(ctx, draft, industry)
		if err == nil && len(r.Questions) < 3 {
			err = errors.New("ai: получено меньше 3 вопросов")
		}
		return r, err
	})
}

func (f *fallback) Card(ctx context.Context, draft, industry string, qs []model.Question) (model.CardResult, error) {
	return run(f, func(b backend) (model.CardResult, error) {
		return b.card(ctx, draft, industry, qs)
	})
}

// NextQuestion: при достижении потолка (5, по more — 8) — done без вызова модели; иначе backend + правила finishNext.
func (f *fallback) NextQuestion(ctx context.Context, draft, industry string, asked []model.Question, more bool) (model.NextQuestionResult, error) {
	if len(asked) >= questionCap(more) {
		return finishNext(model.NextQuestionResult{}, asked, draft, more), nil
	}
	return run(f, func(b backend) (model.NextQuestionResult, error) {
		r, err := b.nextQuestion(ctx, draft, industry, asked, more)
		if err != nil {
			return r, err
		}
		return finishNext(r, asked, draft, more), nil
	})
}

func run[T any](f *fallback, call func(backend) (T, error)) (T, error) {
	if f.primary != nil {
		var err error
		for attempt := 0; attempt < 2; attempt++ {
			var r T
			if r, err = call(f.primary); err == nil {
				f.set(model.AIModeOpenAI, nil)
				return r, nil
			}
		}
		f.set(model.AIModeMock, err)
	} else {
		f.set(model.AIModeMock, nil)
	}
	return call(f.backup)
}

func (f *fallback) set(mode model.AIMode, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mode = mode
	if err != nil || mode == model.AIModeOpenAI {
		f.lastErr = err
	}
}

func (f *fallback) Mode() model.AIMode {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.mode
}

func (f *fallback) Info() Info {
	f.mu.Lock()
	defer f.mu.Unlock()
	info := Info{Mode: f.mode, PromptQuestions: PromptQuestions, PromptCard: PromptCard, PromptNext: PromptNextQuestion, PromptOptions: PromptResultOptions, SchemaExample: SchemaExample, LastCall: lastCallCopy()}
	if f.lastErr != nil {
		s := f.lastErr.Error()
		info.LastError = &s
	}
	return info
}
