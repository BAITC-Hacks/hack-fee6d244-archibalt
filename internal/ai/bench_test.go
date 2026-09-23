package ai

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// Сравнение моделей на реальном пути вопросы→ответы→карточка→Guard.
// Запуск: AI_BENCH=1 OPENAI_API_KEY=... go test ./internal/ai -run TestBenchModels -v
func TestBenchModels(t *testing.T) {
	key := os.Getenv("OPENAI_API_KEY")
	if os.Getenv("AI_BENCH") == "" || key == "" {
		t.Skip("AI_BENCH и OPENAI_API_KEY не заданы")
	}
	models := strings.Split(os.Getenv("AI_BENCH_MODELS"), ",")
	if models[0] == "" {
		models = []string{"gpt-5.4-mini", "gpt-5.5", "gpt-6-luna", "gpt-6-sol", "gpt-4.1-mini"}
	}
	drafts := []struct{ industry, text string }{
		{"ЖКХ", "Хотим автоматизировать приём заявок на ремонт от жильцов, сейчас звонят диспетчеру и всё теряется"},
		{"Ритейл", "Нужно понять, почему падают продажи в двух магазинах из десяти"},
		{"Образование", "Сделать чат-бота для абитуриентов колледжа"},
	}
	answers := map[model.FieldKey]string{
		model.FieldData:              "Есть выгрузка из Excel за 2 года, передадим после NDA в первую неделю",
		model.FieldSuccessCriteria:   "Снизить время обработки с 2 дней до 4 часов",
		model.FieldContact:           "ops@company.kz, созвон раз в неделю по средам",
		model.FieldUsers:             "Диспетчеры и жильцы 40 домов",
		model.FieldConstraints:       "Срок 2 месяца, без доступа к серверу, только тестовые данные",
		model.FieldExpectedResult:    "Прототип веб-формы с дашбордом статусов",
		model.FieldInteractionFormat: "Созвон раз в неделю, вопросы в Telegram",
		model.FieldContext:           "Один диспетчер принимает всё по телефону",
		model.FieldNeed:              "Перестать терять заявки",
		model.FieldTitle:             "Приём заявок",
	}
	for _, m := range models {
		var lat time.Duration
		var nq, filled, cut, errs int
		for _, d := range drafts {
			c := New(key, m)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			s := time.Now()
			q, err := c.Questions(ctx, d.text, d.industry)
			if err != nil || c.Mode() != model.AIModeOpenAI {
				errs++
				cancel()
				continue
			}
			nq += len(q.Questions)
			qs := q.Questions
			for i := range qs {
				qs[i].Answer = answers[qs[i].FieldKey]
			}
			card, err := c.Card(ctx, d.text, d.industry, qs)
			lat += time.Since(s)
			cancel()
			if err != nil || c.Mode() != model.AIModeOpenAI {
				errs++
				continue
			}
			for k, v := range card.Fields {
				if v != "" {
					filled++
				}
				_ = k
			}
			// сколько полей Guard вырезал: сравниваем с «сырой» карточкой нельзя (guard внутри), поэтому считаем пустые поля при заданных ответах
			for i := range qs {
				if qs[i].Answer != "" && card.Fields[qs[i].FieldKey] == "" {
					cut++
				}
			}
		}
		fmt.Printf("%-14s | ошибок %d | ср. задержка на задачу %.1fs | вопросов всего %d | заполнено полей %d | вырезано Guard при данном ответе %d\n",
			m, errs, lat.Seconds()/float64(len(drafts)), nq, filled, cut)
	}
}
