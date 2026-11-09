package search

import (
	"context"
	"fmt"

	"era/booru/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SearchMedia(ctx context.Context, pool *pgxpool.Pool, expr string, limit int, cursor string) ([]db.Media, string, error) {
	where, args := WhereSQL(expr, 1)
	conditions := []string{"m.status = 'ready'"}
	if where != "" {
		conditions = append(conditions, where)
	}
	position, err := decodeCursor[mediaCursor](cursor)
	if err != nil {
		return nil, "", err
	}
	if !position.UploadedAt.IsZero() {
		args = append(args, position.UploadedAt, position.ID)
		conditions = append(conditions, fmt.Sprintf("(m.uploaded_at, m.id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	args = append(args, limit+1)
	query := fmt.Sprintf(`SELECT m.id, m.original_name, m.format, m.width, m.height, m.duration, m.size, m.uploaded_at
FROM media m WHERE %s ORDER BY m.uploaded_at DESC, m.id DESC LIMIT $%d`, joinConditions(conditions), len(args))
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	items, err := scanMedia(rows)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		next = encodeCursor(mediaCursor{UploadedAt: last.UploadedAt, ID: last.ID})
	}
	return items, next, nil
}

func scanMedia(rows pgx.Rows) ([]db.Media, error) {
	defer rows.Close()
	items := []db.Media{}
	for rows.Next() {
		var item db.Media
		if err := rows.Scan(&item.ID, &item.OriginalName, &item.Format, &item.Width, &item.Height, &item.Duration, &item.Size, &item.UploadedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
