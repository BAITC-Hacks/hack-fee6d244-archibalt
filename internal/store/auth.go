package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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
