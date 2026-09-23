package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// Форматы seed-файлов — раздел «Формат seed-файлов» в API_CONTRACT.md.

type seedTeam struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Skills    []string `json:"skills"`
	Interests []string `json:"interests"`
	Tech      []string `json:"tech"`
	Points    int      `json:"points"`
}

type seedQuestion struct {
	Text     string         `json:"text"`
	FieldKey model.FieldKey `json:"field_key"`
	Answer   string         `json:"answer"`
}

type seedTask struct {
	ID        int               `json:"id"`
	Industry  string            `json:"industry"`
	DraftText string            `json:"draft_text"`
	Fields    map[string]string `json:"fields"`
	Confirmed bool              `json:"confirmed"`
	Status    string            `json:"status"`
	Questions []seedQuestion    `json:"questions"`
}

type seedProposal struct {
	ID             int    `json:"id"`
	TaskID         int    `json:"task_id"`
	TeamID         int    `json:"team_id"`
	Idea           string `json:"idea"`
	Plan           string `json:"plan"`
	Deadline       string `json:"deadline"`
	Link           string `json:"link"`
	Status         string `json:"status"`
	StageConfirmed bool   `json:"stage_confirmed"`
}

// readSeed читает JSON-массив; отсутствующий файл — не ошибка (ok=false).
func readSeed[T any](dir, name string) (items []T, ok bool, err error) {
	path := filepath.Join(dir, name)
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("seed %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, false, fmt.Errorf("seed %s: ожидается JSON-массив объектов: %w", path, err)
	}
	return items, true, nil
}

func checkIDs[T any](file string, items []T, id func(T) int) (map[int]bool, error) {
	seen := make(map[int]bool, len(items))
	for i, it := range items {
		v := id(it)
		if v <= 0 {
			return nil, fmt.Errorf("seed %s: элемент #%d: id должен быть положительным целым, получено %d", file, i, v)
		}
		if seen[v] {
			return nil, fmt.Errorf("seed %s: дублируется id %d", file, v)
		}
		seen[v] = true
	}
	return seen, nil
}

var knownFields = func() map[model.FieldKey]bool {
	m := map[model.FieldKey]bool{}
	for _, k := range model.FieldKeys {
		m[k] = true
	}
	return m
}()

// SeedIfEmpty загружает seed/teams.json → tasks.json → proposals.json, если таблица tasks пуста.
// Отсутствующие файлы пропускаются с предупреждением. Рассинхрон FK / дубли id — ошибка.
func (s *Store) SeedIfEmpty(ctx context.Context, dir string, compute func(model.Fields, bool) model.Rating) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM tasks`).Scan(&n); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	if n > 0 {
		slog.Info("seed: таблица tasks не пуста, пропускаю", "tasks", n)
		return nil
	}

	teams, teamsOK, err := readSeed[seedTeam](dir, "teams.json")
	if err != nil {
		return err
	}
	tasks, tasksOK, err := readSeed[seedTask](dir, "tasks.json")
	if err != nil {
		return err
	}
	props, propsOK, err := readSeed[seedProposal](dir, "proposals.json")
	if err != nil {
		return err
	}
	for name, ok := range map[string]bool{"teams.json": teamsOK, "tasks.json": tasksOK, "proposals.json": propsOK} {
		if !ok {
			slog.Warn("seed: файл не найден, пропускаю", "file", filepath.Join(dir, name))
		}
	}

	teamIDs, err := checkIDs("teams.json", teams, func(t seedTeam) int { return t.ID })
	if err != nil {
		return err
	}
	taskIDs, err := checkIDs("tasks.json", tasks, func(t seedTask) int { return t.ID })
	if err != nil {
		return err
	}
	if _, err := checkIDs("proposals.json", props, func(p seedProposal) int { return p.ID }); err != nil {
		return err
	}
	for _, p := range props {
		if !taskIDs[p.TaskID] {
			return fmt.Errorf("seed proposals.json: отклик %d ссылается на task_id %d, которого нет в tasks.json", p.ID, p.TaskID)
		}
		if !teamIDs[p.TeamID] {
			return fmt.Errorf("seed proposals.json: отклик %d ссылается на team_id %d, которого нет в teams.json", p.ID, p.TeamID)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, t := range teams {
		// ON CONFLICT: команды могли загрузиться раньше, когда tasks.json ещё не было.
		if _, err := tx.ExecContext(ctx, `INSERT INTO teams (id, name, skills, interests, tech, points)
			VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO NOTHING`,
			t.ID, t.Name, nonNil(t.Skills), nonNil(t.Interests), nonNil(t.Tech), t.Points); err != nil {
			return fmt.Errorf("seed team %d: %w", t.ID, err)
		}
	}

	now := time.Now()
	for _, st := range tasks {
		t, err := st.toTask(now, compute)
		if err != nil {
			return err
		}
		j, err := encodeTask(&t)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO tasks
			(id, industry, status, draft_text, fields, confirmed, score, rating, questions, ai_mode, created_at, published_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			t.ID, t.Industry, string(t.Status), t.DraftText, string(j.fields), t.Confirmed, t.Score,
			string(j.rating), string(j.questions), string(t.AIMode), t.CreatedAt, t.PublishedAt); err != nil {
			return fmt.Errorf("seed task %d: %w", t.ID, err)
		}
	}

	for _, p := range props {
		status := model.ProposalStatus(p.Status)
		switch status {
		case "":
			status = model.ProposalNew
		case model.ProposalNew, model.ProposalSelected, model.ProposalRejected:
		default:
			slog.Warn("seed: неизвестный статус отклика, ставлю new", "proposal", p.ID, "status", p.Status)
			status = model.ProposalNew
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO proposals
			(id, task_id, team_id, idea, plan, deadline, link, status, stage_confirmed)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			p.ID, p.TaskID, p.TeamID, p.Idea, p.Plan, p.Deadline, p.Link, string(status), p.StageConfirmed); err != nil {
			return fmt.Errorf("seed proposal %d: %w", p.ID, err)
		}
	}

	if err := resetSequences(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	slog.Info("seed загружен", "dir", dir, "teams", len(teams), "tasks", len(tasks), "proposals", len(props))
	return nil
}

func (st seedTask) toTask(now time.Time, compute func(model.Fields, bool) model.Rating) (model.Task, error) {
	fields := model.Fields{}
	for k, v := range st.Fields {
		if !knownFields[model.FieldKey(k)] {
			slog.Warn("seed: неизвестное поле карточки, пропускаю", "task", st.ID, "field", k)
			continue
		}
		fields[model.FieldKey(k)] = v
	}
	fields = fields.Full()

	t := model.Task{
		ID:        st.ID,
		Industry:  st.Industry,
		DraftText: st.DraftText,
		Fields:    fields,
		Confirmed: st.Confirmed,
		Rating:    compute(fields, st.Confirmed),
		AIMode:    model.AIModeMock,
		CreatedAt: now.Add(-time.Duration(st.ID) * time.Minute),
		Questions: make([]model.Question, 0, len(st.Questions)),
	}
	for i, q := range st.Questions {
		t.Questions = append(t.Questions, model.Question{ID: i + 1, Text: q.Text, FieldKey: q.FieldKey, Answer: q.Answer})
	}

	switch {
	case st.Confirmed:
		if st.Status != "" && st.Status != string(model.StatusPublished) {
			slog.Warn("seed: confirmed=true, статус заменён на published", "task", st.ID, "status", st.Status)
		}
		t.Status = model.StatusPublished
		pub := t.CreatedAt
		t.PublishedAt = &pub
	case st.Status == "" || st.Status == string(model.StatusEditing):
		t.Status = model.StatusEditing
	default:
		// опубликовать можно только подтверждённую карточку (ТЗ §3 — ручное подтверждение)
		slog.Warn("seed: confirmed=false, статус заменён на editing", "task", st.ID, "status", st.Status)
		t.Status = model.StatusEditing
	}
	return t, nil
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// resetSequences выставляет serial-последовательности на max(id) после вставки с явными id.
func resetSequences(ctx context.Context, tx *sql.Tx) error {
	for _, table := range []string{"teams", "tasks", "proposals"} {
		q := fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%[1]s', 'id'),
			GREATEST(COALESCE(MAX(id), 0), 1), MAX(id) IS NOT NULL) FROM %[1]s`, table)
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("setval %s: %w", table, err)
		}
	}
	return nil
}
