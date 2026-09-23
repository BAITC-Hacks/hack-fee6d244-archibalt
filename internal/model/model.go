// Package model — общие типы API (см. API_CONTRACT.md). Единственный источник правды для JSON-формы.
package model

import "time"

type FieldKey string

const (
	FieldTitle             FieldKey = "title"
	FieldContext           FieldKey = "context"
	FieldNeed              FieldKey = "need"
	FieldUsers             FieldKey = "users"
	FieldData              FieldKey = "data"
	FieldConstraints       FieldKey = "constraints"
	FieldExpectedResult    FieldKey = "expected_result"
	FieldSuccessCriteria   FieldKey = "success_criteria"
	FieldContact           FieldKey = "contact"
	FieldInteractionFormat FieldKey = "interaction_format"
)

// FieldKeys — порядок полей карточки по ТЗ §3.
var FieldKeys = []FieldKey{FieldTitle, FieldContext, FieldNeed, FieldUsers, FieldData, FieldConstraints, FieldExpectedResult, FieldSuccessCriteria, FieldContact, FieldInteractionFormat}

// FieldLabels — подписи на русском для UI и AI-промптов.
var FieldLabels = map[FieldKey]string{
	FieldTitle: "Название", FieldContext: "Контекст", FieldNeed: "Потребность", FieldUsers: "Пользователи",
	FieldData: "Данные и материалы", FieldConstraints: "Ограничения", FieldExpectedResult: "Ожидаемый результат",
	FieldSuccessCriteria: "Критерии успеха", FieldContact: "Контакт", FieldInteractionFormat: "Формат взаимодействия",
}

type Fields map[FieldKey]string

// Full возвращает копию со всеми 10 ключами (пустые = "").
func (f Fields) Full() Fields {
	out := make(Fields, len(FieldKeys))
	for _, k := range FieldKeys {
		out[k] = f[k]
	}
	return out
}

type Level string

const (
	LevelDraft    Level = "draft"    // 0–39
	LevelWorking  Level = "working"  // 40–69
	LevelReady    Level = "ready"    // 70–89
	LevelPriority Level = "priority" // 90–100
)

var LevelLabels = map[Level]string{LevelDraft: "черновик", LevelWorking: "рабочая", LevelReady: "готовая", LevelPriority: "приоритетная"}

type TaskStatus string

const (
	StatusDraft      TaskStatus = "draft"
	StatusClarifying TaskStatus = "clarifying"
	StatusEditing    TaskStatus = "editing"
	StatusPublished  TaskStatus = "published"
)

type BreakdownItem struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Weight int    `json:"weight"`
	Earned int    `json:"earned"`
	Reason string `json:"reason"`
}

type MissingItem struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Gain  int    `json:"gain"`
	Hint  string `json:"hint"`
}

// Rating — результат internal/rating.Compute; хранится в tasks как json + score.
type Rating struct {
	Score      int             `json:"score"`
	Level      Level           `json:"level"`
	LevelLabel string          `json:"level_label"`
	Breakdown  []BreakdownItem `json:"breakdown"`
	Missing    []MissingItem   `json:"missing"`
}

type Question struct {
	ID       int      `json:"id"`
	Text     string   `json:"text"`
	FieldKey FieldKey `json:"field_key"`
	Answer   string   `json:"answer"`
}

type AIMode string

const (
	AIModeOpenAI AIMode = "openai"
	AIModeMock   AIMode = "mock"
)

type Task struct {
	ID          int        `json:"id"`
	Industry    string     `json:"industry"`
	Status      TaskStatus `json:"status"`
	DraftText   string     `json:"draft_text"`
	Fields      Fields     `json:"fields"`
	Confirmed   bool       `json:"confirmed"`
	Rating                 // score, level, level_label, breakdown, missing — плоско в JSON
	Questions   []Question `json:"questions"`
	AIMode      AIMode     `json:"ai_mode"`
	CreatedAt   time.Time  `json:"created_at"`
	PublishedAt *time.Time `json:"published_at"`
	Proposals   []Proposal `json:"proposals,omitempty"`
}

type ProposalStatus string

const (
	ProposalNew      ProposalStatus = "new"
	ProposalSelected ProposalStatus = "selected"
	ProposalRejected ProposalStatus = "rejected"
)

type TeamRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Proposal struct {
	ID             int            `json:"id"`
	TaskID         int            `json:"task_id"`
	Team           TeamRef        `json:"team"`
	Idea           string         `json:"idea"`
	Plan           string         `json:"plan"`
	Deadline       string         `json:"deadline"`
	Link           string         `json:"link"`
	Status         ProposalStatus `json:"status"`
	StageConfirmed bool           `json:"stage_confirmed"`
	CreatedAt      time.Time      `json:"created_at"`
}

type Team struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Skills    []string `json:"skills"`
	Interests []string `json:"interests"`
	Tech      []string `json:"tech"`
	Points    int      `json:"points"`
	Contact   string   `json:"contact"` // нормализованный email/телефон для входа; пусто у seed-команд
}

// ---- контракты между модулями ----

// QuestionsResult — ответ AI на черновик.
type QuestionsResult struct {
	Questions     []Question `json:"questions"` // ≥3, Answer пустой
	MissingFields []FieldKey `json:"missing_fields"`
}

// CardResult — ответ AI: поля только из слов пользователя, пустые = "".
type CardResult struct {
	Fields Fields `json:"fields"`
}
