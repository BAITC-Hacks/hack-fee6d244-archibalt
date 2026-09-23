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

// ProposalCounts — число откликов по задачам (для карточек каталога).
func (s *Store) ProposalCounts(ctx context.Context) (map[int]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT task_id, count(*) FROM proposals GROUP BY task_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]int{}
	for rows.Next() {
		var id, n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}
