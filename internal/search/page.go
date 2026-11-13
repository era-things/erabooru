package search

import (
	"era/booru/internal/db"
	"github.com/jackc/pgx/v5"
)

// collectMediaPage owns row cleanup and the extra-row pagination convention.
// Each query supplies its projection and the cursor for its ordering, so time
// and distance searches share pagination without changing their SQL plans.
func collectMediaPage[C any](rows pgx.Rows, limit int, scan func(pgx.Row, *db.Media) (C, error)) ([]db.Media, string, error) {
	defer rows.Close()
	items := []db.Media{}
	if limit <= 0 {
		return items, "", nil
	}
	var last C
	for rows.Next() {
		var item db.Media
		cursor, err := scan(rows, &item)
		if err != nil {
			return nil, "", err
		}
		items = append(items, item)
		if len(items) <= limit {
			last = cursor
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(items) > limit {
		return items[:limit], encodeCursor(last), nil
	}
	return items, "", nil
}
