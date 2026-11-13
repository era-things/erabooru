package search

import (
	"context"
	"fmt"
	"strings"

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
FROM media m WHERE %s ORDER BY m.uploaded_at DESC, m.id DESC LIMIT $%d`, strings.Join(conditions, " AND "), len(args))
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	return collectMediaPage(rows, limit, func(row pgx.Row, item *db.Media) (mediaCursor, error) {
		err := row.Scan(&item.ID, &item.OriginalName, &item.Format, &item.Width, &item.Height, &item.Duration, &item.Size, &item.UploadedAt)
		return mediaCursor{UploadedAt: item.UploadedAt, ID: item.ID}, err
	})
}
