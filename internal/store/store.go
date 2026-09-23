// Package store — доступ к Postgres: задачи, команды, отклики, seed.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // драйвер "pgx" для database/sql

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/db"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// ErrNotFound — запись с таким id не существует.
var ErrNotFound = errors.New("not found")

type Store struct{ db *sql.DB }

// Open подключается к Postgres, повторяя попытки до 30 с (в compose база стартует не сразу).
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL не задан")
	}
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	deadline := time.Now().Add(30 * time.Second)
	for attempt := 1; ; attempt++ {
		pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = sqlDB.PingContext(pctx)
		cancel()
		if err == nil {
			return &Store{db: sqlDB}, nil
		}
		if time.Now().After(deadline) {
			sqlDB.Close()
			return nil, fmt.Errorf("postgres недоступен за 30 с: %w", err)
		}
		slog.Warn("postgres ещё не готов, повтор", "attempt", attempt, "err", err)
		select {
		case <-ctx.Done():
			sqlDB.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Migrate применяет db/schema.sql (идемпотентно, CREATE ... IF NOT EXISTS).
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, db.Schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// ---- задачи ----

const taskCols = `id, industry, status, draft_text, fields, confirmed, rating, questions, ai_mode, created_at, published_at`

type scanner interface{ Scan(dest ...any) error }

func scanTask(sc scanner) (model.Task, error) {
	var (
		t                         model.Task
		status, aiMode            string
		fields, rating, questions []byte
		published                 sql.NullTime
	)
	if err := sc.Scan(&t.ID, &t.Industry, &status, &t.DraftText, &fields, &t.Confirmed, &rating, &questions, &aiMode, &t.CreatedAt, &published); err != nil {
		return t, err
	}
	t.Status, t.AIMode = model.TaskStatus(status), model.AIMode(aiMode)
	if err := json.Unmarshal(fields, &t.Fields); err != nil {
		return t, fmt.Errorf("task %d fields: %w", t.ID, err)
	}
	t.Fields = t.Fields.Full()
	if len(rating) > 0 {
		if err := json.Unmarshal(rating, &t.Rating); err != nil {
			return t, fmt.Errorf("task %d rating: %w", t.ID, err)
		}
	}
	if t.Breakdown == nil {
		t.Breakdown = []model.BreakdownItem{}
	}
	if t.Missing == nil {
		t.Missing = []model.MissingItem{}
	}
	if err := json.Unmarshal(questions, &t.Questions); err != nil {
		return t, fmt.Errorf("task %d questions: %w", t.ID, err)
	}
	if t.Questions == nil {
		t.Questions = []model.Question{}
	}
	if published.Valid {
		pt := published.Time
		t.PublishedAt = &pt
	}
	return t, nil
}

// ListTasks — опубликованные задачи (score DESC, published_at ASC) с опциональными
// фильтрами по отрасли и уровню, плюс список всех отраслей каталога для фильтра.
func (s *Store) ListTasks(ctx context.Context, industry, level string) ([]model.Task, []string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+taskCols+` FROM tasks
		WHERE status = 'published'
		  AND ($1 = '' OR industry = $1)
		  AND ($2 = '' OR rating->>'level' = $2)
		ORDER BY score DESC, published_at ASC NULLS LAST, id ASC`, industry, level)
	if err != nil {
		return nil, nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	tasks := []model.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, nil, err
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	irows, err := s.db.QueryContext(ctx, `SELECT DISTINCT industry FROM tasks WHERE status = 'published' AND industry <> '' ORDER BY industry`)
	if err != nil {
		return nil, nil, fmt.Errorf("list industries: %w", err)
	}
	defer irows.Close()
	industries := []string{}
	for irows.Next() {
		var ind string
		if err := irows.Scan(&ind); err != nil {
			return nil, nil, err
		}
		industries = append(industries, ind)
	}
	return tasks, industries, irows.Err()
}

// GetTask — задача любого статуса вместе с откликами.
func (s *Store) GetTask(ctx context.Context, id int) (model.Task, error) {
	t, err := scanTask(s.db.QueryRowContext(ctx, `SELECT `+taskCols+` FROM tasks WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, fmt.Errorf("get task: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, proposalSelect+` WHERE p.task_id = $1 ORDER BY p.created_at, p.id`, id)
	if err != nil {
		return t, fmt.Errorf("list proposals: %w", err)
	}
	defer rows.Close()
	t.Proposals = []model.Proposal{}
	for rows.Next() {
		p, err := scanProposal(rows)
		if err != nil {
			return t, err
		}
		t.Proposals = append(t.Proposals, p)
	}
	return t, rows.Err()
}

type taskJSON struct{ fields, rating, questions []byte }

func encodeTask(t *model.Task) (taskJSON, error) {
	var (
		j   taskJSON
		err error
	)
	if j.fields, err = json.Marshal(t.Fields.Full()); err != nil {
		return j, err
	}
	if j.rating, err = json.Marshal(t.Rating); err != nil {
		return j, err
	}
	qs := t.Questions
	if qs == nil {
		qs = []model.Question{}
	}
	j.questions, err = json.Marshal(qs)
	return j, err
}

// CreateTask вставляет задачу и заполняет t.ID и t.CreatedAt.
func (s *Store) CreateTask(ctx context.Context, t *model.Task) error {
	j, err := encodeTask(t)
	if err != nil {
		return err
	}
	err = s.db.QueryRowContext(ctx, `INSERT INTO tasks
		(industry, status, draft_text, fields, confirmed, score, rating, questions, ai_mode, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id, created_at`,
		t.Industry, string(t.Status), t.DraftText, string(j.fields), t.Confirmed, t.Score, string(j.rating),
		string(j.questions), string(t.AIMode), t.PublishedAt,
	).Scan(&t.ID, &t.CreatedAt)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

// UpdateTask сохраняет изменяемые поля задачи (fields, status, confirmed, rating, questions, ai_mode, published_at).
func (s *Store) UpdateTask(ctx context.Context, t *model.Task) error {
	j, err := encodeTask(t)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE tasks SET
		fields = $2, status = $3, confirmed = $4, score = $5, rating = $6, questions = $7, ai_mode = $8, published_at = $9
		WHERE id = $1`,
		t.ID, string(j.fields), string(t.Status), t.Confirmed, t.Score, string(j.rating), string(j.questions),
		string(t.AIMode), t.PublishedAt)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- команды ----

const teamSelect = `SELECT id, name, to_jsonb(skills), to_jsonb(interests), to_jsonb(tech), points FROM teams`

func scanTeam(sc scanner) (model.Team, error) {
	var (
		t                       model.Team
		skills, interests, tech []byte
	)
	if err := sc.Scan(&t.ID, &t.Name, &skills, &interests, &tech, &t.Points); err != nil {
		return t, err
	}
	for _, p := range []struct {
		raw []byte
		dst *[]string
	}{{skills, &t.Skills}, {interests, &t.Interests}, {tech, &t.Tech}} {
		if err := json.Unmarshal(p.raw, p.dst); err != nil {
			return t, fmt.Errorf("team %d: %w", t.ID, err)
		}
		if *p.dst == nil {
			*p.dst = []string{}
		}
	}
	return t, nil
}

func (s *Store) ListTeams(ctx context.Context) ([]model.Team, error) {
	rows, err := s.db.QueryContext(ctx, teamSelect+` ORDER BY points DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	defer rows.Close()
	teams := []model.Team{}
	for rows.Next() {
		t, err := scanTeam(rows)
		if err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

func (s *Store) GetTeam(ctx context.Context, id int) (model.Team, error) {
	t, err := scanTeam(s.db.QueryRowContext(ctx, teamSelect+` WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, fmt.Errorf("get team: %w", err)
	}
	return t, nil
}

// ---- отклики ----

const proposalSelect = `SELECT p.id, p.task_id, p.team_id, tm.name, p.idea, p.plan, p.deadline, p.link,
	p.status, p.stage_confirmed, p.created_at FROM proposals p JOIN teams tm ON tm.id = p.team_id`

func scanProposal(sc scanner) (model.Proposal, error) {
	var (
		p      model.Proposal
		status string
	)
	err := sc.Scan(&p.ID, &p.TaskID, &p.Team.ID, &p.Team.Name, &p.Idea, &p.Plan, &p.Deadline, &p.Link,
		&status, &p.StageConfirmed, &p.CreatedAt)
	p.Status = model.ProposalStatus(status)
	return p, err
}

// CreateProposal вставляет отклик (p.TaskID, p.Team.ID обязательны) и возвращает его полностью.
func (s *Store) CreateProposal(ctx context.Context, p *model.Proposal) error {
	if p.Status == "" {
		p.Status = model.ProposalNew
	}
	var id int
	err := s.db.QueryRowContext(ctx, `INSERT INTO proposals (task_id, team_id, idea, plan, deadline, link, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		p.TaskID, p.Team.ID, p.Idea, p.Plan, p.Deadline, p.Link, string(p.Status)).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation: нет задачи или команды
			return ErrNotFound
		}
		return fmt.Errorf("create proposal: %w", err)
	}
	got, err := s.GetProposal(ctx, id)
	if err != nil {
		return err
	}
	*p = got
	return nil
}

func (s *Store) GetProposal(ctx context.Context, id int) (model.Proposal, error) {
	p, err := scanProposal(s.db.QueryRowContext(ctx, proposalSelect+` WHERE p.id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, fmt.Errorf("get proposal: %w", err)
	}
	return p, nil
}

func (s *Store) UpdateProposalStatus(ctx context.Context, id int, status model.ProposalStatus) (model.Proposal, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE proposals SET status = $2 WHERE id = $1`, id, string(status))
	if err != nil {
		return model.Proposal{}, fmt.Errorf("update proposal: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.Proposal{}, ErrNotFound
	}
	return s.GetProposal(ctx, id)
}

// StagePoints — баллы команде за подтверждённый этап.
const StagePoints = 10

// ConfirmStage отмечает этап подтверждённым и начисляет команде StagePoints в одной транзакции.
// Идемпотентно: повторный вызов баллы не начисляет.
func (s *Store) ConfirmStage(ctx context.Context, id int) (model.Proposal, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Proposal{}, err
	}
	defer tx.Rollback()

	var teamID int
	err = tx.QueryRowContext(ctx, `UPDATE proposals SET stage_confirmed = true
		WHERE id = $1 AND NOT stage_confirmed RETURNING team_id`, id).Scan(&teamID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// либо уже подтверждено, либо нет такого отклика — GetProposal различит
	case err != nil:
		return model.Proposal{}, fmt.Errorf("confirm stage: %w", err)
	default:
		if _, err := tx.ExecContext(ctx, `UPDATE teams SET points = points + $2 WHERE id = $1`, teamID, StagePoints); err != nil {
			return model.Proposal{}, fmt.Errorf("add points: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Proposal{}, err
	}
	return s.GetProposal(ctx, id)
}
