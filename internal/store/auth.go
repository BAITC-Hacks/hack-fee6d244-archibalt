package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// GetTeamByContact ищет команду по контакту без учёта регистра.
func (s *Store) GetTeamByContact(ctx context.Context, contact string) (model.Team, error) {
	return s.getTeamWhere(ctx, ` WHERE contact <> '' AND lower(contact) = lower($1) ORDER BY id LIMIT 1`, strings.TrimSpace(contact))
}

// GetTeamByName ищет команду по имени без учёта регистра и крайних пробелов (первую по id).
func (s *Store) GetTeamByName(ctx context.Context, name string) (model.Team, error) {
	return s.getTeamWhere(ctx, ` WHERE lower(btrim(name)) = lower(btrim($1)) ORDER BY id LIMIT 1`, name)
}

func (s *Store) getTeamWhere(ctx context.Context, where string, arg any) (model.Team, error) {
	t, err := scanTeam(s.db.QueryRowContext(ctx, teamSelect+where, arg))
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, fmt.Errorf("get team: %w", err)
	}
	return t, nil
}

// CreateTeam вставляет команду (name, contact; навыки пустые, 0 баллов) и заполняет t целиком.
func (s *Store) CreateTeam(ctx context.Context, t *model.Team) error {
	var id int
	err := s.db.QueryRowContext(ctx, `INSERT INTO teams (name, contact, skills, interests, tech, points)
		VALUES ($1, $2, $3, $4, $5, 0) RETURNING id`,
		t.Name, t.Contact, nonNil(t.Skills), nonNil(t.Interests), nonNil(t.Tech)).Scan(&id)
	if err != nil {
		return fmt.Errorf("create team: %w", err)
	}
	got, err := s.GetTeam(ctx, id)
	if err != nil {
		return err
	}
	*t = got
	return nil
}

// SetTeamContact привязывает контакт к команде, только если он у неё ещё пуст (иначе ErrNotFound).
func (s *Store) SetTeamContact(ctx context.Context, id int, contact string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE teams SET contact = $2 WHERE id = $1 AND COALESCE(contact, '') = ''`, id, contact)
	if err != nil {
		return fmt.Errorf("set team contact: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListProposalsByTeam — отклики команды, новые сверху.
func (s *Store) ListProposalsByTeam(ctx context.Context, teamID int) ([]model.Proposal, error) {
	rows, err := s.db.QueryContext(ctx, proposalSelect+` WHERE p.team_id = $1 ORDER BY p.created_at DESC, p.id DESC`, teamID)
	if err != nil {
		return nil, fmt.Errorf("list team proposals: %w", err)
	}
	defer rows.Close()
	out := []model.Proposal{}
	for rows.Next() {
		p, err := scanProposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListTasksByOwner — задачи заявителя с этим контактом (любого статуса, новые сверху), каждая с откликами.
func (s *Store) ListTasksByOwner(ctx context.Context, contact string) ([]model.Task, error) {
	contact = strings.TrimSpace(contact)
	rows, err := s.db.QueryContext(ctx, `SELECT `+taskCols+` FROM tasks
		WHERE owner_contact <> '' AND lower(owner_contact) = lower($1) ORDER BY created_at DESC, id DESC`, contact)
	if err != nil {
		return nil, fmt.Errorf("list owner tasks: %w", err)
	}
	defer rows.Close()
	tasks := []model.Task{}
	idx := map[int]int{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		t.Proposals = []model.Proposal{}
		idx[t.ID] = len(tasks)
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return tasks, nil
	}
	prows, err := s.db.QueryContext(ctx, proposalSelect+` JOIN tasks t ON t.id = p.task_id
		WHERE t.owner_contact <> '' AND lower(t.owner_contact) = lower($1) ORDER BY p.created_at, p.id`, contact)
	if err != nil {
		return nil, fmt.Errorf("list owner proposals: %w", err)
	}
	defer prows.Close()
	for prows.Next() {
		p, err := scanProposal(prows)
		if err != nil {
			return nil, err
		}
		if i, ok := idx[p.TaskID]; ok { // задача могла появиться между запросами — её отклики пропускаем
			tasks[i].Proposals = append(tasks[i].Proposals, p)
		}
	}
	return tasks, prows.Err()
}

// SetTaskOwner задаёт контакт заявителя задачи (права проверяет хендлер).
func (s *Store) SetTaskOwner(ctx context.Context, id int, contact string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE tasks SET owner_contact = $2 WHERE id = $1`, id, contact)
	if err != nil {
		return fmt.Errorf("set task owner: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListMessages — чат отклика по времени (старые сверху).
func (s *Store) ListMessages(ctx context.Context, proposalID int) ([]model.Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, author, text, created_at FROM messages
		WHERE proposal_id = $1 ORDER BY created_at, id`, proposalID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()
	out := []model.Message{}
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.Author, &m.Text, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AddMessage сохраняет сообщение чата (author: model.AuthorTeam | model.AuthorBusiness); нет отклика → ErrNotFound.
func (s *Store) AddMessage(ctx context.Context, proposalID int, author, text string) (model.Message, error) {
	m := model.Message{Author: author, Text: text}
	err := s.db.QueryRowContext(ctx, `INSERT INTO messages (proposal_id, author, text) VALUES ($1, $2, $3)
		RETURNING id, created_at`, proposalID, author, text).Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return m, ErrNotFound
		}
		return m, fmt.Errorf("add message: %w", err)
	}
	return m, nil
}
