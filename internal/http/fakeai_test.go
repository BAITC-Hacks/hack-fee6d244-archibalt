package httpapi

import (
	"context"
	"strings"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/ai"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// fakeAI — детерминированная реализация ai.Client: 3 вопроса, карточка = черновик + ответы по полям.
type fakeAI struct{}

func (fakeAI) Questions(context.Context, string, string) (model.QuestionsResult, error) {
	keys := []model.FieldKey{model.FieldUsers, model.FieldData, model.FieldSuccessCriteria}
	var r model.QuestionsResult
	for _, k := range keys {
		r.Questions = append(r.Questions, model.Question{Text: "Вопрос про " + string(k), FieldKey: k}) // id не задан — хендлер проставит
		r.MissingFields = append(r.MissingFields, k)
	}
	return r, nil
}

func (fakeAI) Card(_ context.Context, draft, _ string, qs []model.Question) (model.CardResult, error) {
	f := model.Fields{model.FieldContext: strings.TrimSpace(draft)}
	for _, q := range qs {
		if q.Answer != "" {
			f[q.FieldKey] = q.Answer
		}
	}
	return model.CardResult{Fields: f.Full()}, nil
}

func (fakeAI) Mode() model.AIMode { return model.AIModeMock }

func (fakeAI) Info() ai.Info {
	return ai.Info{Mode: model.AIModeMock, PromptQuestions: "q", PromptCard: "c", SchemaExample: "{}"}
}
