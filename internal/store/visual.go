package store

import (
	"context"
	"database/sql"
	"errors"
)

// Visual — сохранённый визуальный концепт задачи.
type Visual struct {
	Mime   string
	Data   []byte
	Prompt string
	AIMode string
}

// SaveVisual сохраняет концепт задачи; повторный вызов перезаписывает.
func (s *Store) SaveVisual(ctx context.Context, taskID int, v Visual) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO task_visuals (task_id, mime, data, prompt, ai_mode) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (task_id) DO UPDATE SET mime = EXCLUDED.mime, data = EXCLUDED.data, prompt = EXCLUDED.prompt,
		ai_mode = EXCLUDED.ai_mode, created_at = now()`, taskID, v.Mime, v.Data, v.Prompt, v.AIMode)
	return err
}

// GetVisual — концепт задачи; ErrNotFound, если его нет.
func (s *Store) GetVisual(ctx context.Context, taskID int) (Visual, error) {
	var v Visual
	err := s.db.QueryRowContext(ctx, `SELECT mime, data, prompt, ai_mode FROM task_visuals WHERE task_id = $1`, taskID).
		Scan(&v.Mime, &v.Data, &v.Prompt, &v.AIMode)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}

// HasVisual — есть ли у задачи концепт (без чтения картинки).
func (s *Store) HasVisual(ctx context.Context, taskID int) (bool, error) {
	var ok bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM task_visuals WHERE task_id = $1)`, taskID).Scan(&ok)
	return ok, err
}
