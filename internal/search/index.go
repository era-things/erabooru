package search

import (
	"context"
	"fmt"

	"era/booru/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SearchMedia(ctx context.Context, pool *pgxpool.Pool, expr string, limit, offset int) ([]db.Media, int, error) {
	where, args := WhereSQL(expr, 1)
	if where != "" {
		where = " WHERE " + where
	}
	var total int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM media m`+where, args...).Scan(&total); err != nil || total == 0 {
		return nil, total, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit, offset)
	query := fmt.Sprintf(`SELECT m.id, m.format, m.width, m.height, m.duration, m.size, m.uploaded_at
FROM media m%s ORDER BY m.uploaded_at DESC LIMIT $%d OFFSET $%d`, where, len(args)+1, len(args)+2)
	rows, err := pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	items, err := scanMedia(rows)
	return items, total, err
}

func scanMedia(rows pgx.Rows) ([]db.Media, error) {
	defer rows.Close()
	items := []db.Media{}
	for rows.Next() {
		var item db.Media
		if err := rows.Scan(&item.ID, &item.Format, &item.Width, &item.Height, &item.Duration, &item.Size, &item.UploadedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
