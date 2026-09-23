package store

import (
	"context"
	"fmt"
)

// Stats — «пульс площадки» над каталогом: опубликованные задачи, все отклики, все команды.
type Stats struct {
	Tasks     int `json:"tasks"`
	Proposals int `json:"proposals"`
	Teams     int `json:"teams"`
}

// Stats — три COUNT одним запросом.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM tasks WHERE status = 'published'),
		(SELECT count(*) FROM proposals),
		(SELECT count(*) FROM teams)`).Scan(&st.Tasks, &st.Proposals, &st.Teams)
	if err != nil {
		return st, fmt.Errorf("stats: %w", err)
	}
	return st, nil
}
