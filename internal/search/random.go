package search

import (
	"context"
	"fmt"
	"strings"

	"era/booru/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type randomCursor struct {
	Seed string `json:"seed"`
	Rank string `json:"rank"`
	ID   string `json:"id"`
}

// RandomMedia gives each ID a deterministic random rank for the supplied seed.
// The ID tie-break and seed-bound cursor keep pagination stable.
func RandomMedia(ctx context.Context, pool *pgxpool.Pool, expr, seed string, limit int, cursor string) ([]db.Media, string, error) {
	position, err := decodeCursor[randomCursor](cursor)
	if err != nil {
		return nil, "", err
	}
	if position.ID != "" && position.Seed != seed {
		return nil, "", fmt.Errorf("%w: random seed changed", ErrInvalidCursor)
	}
	args := []any{seed}
	conditions := []string{"m.status='ready'"}
	where, filterArgs := WhereSQL(expr, 2)
	args = append(args, filterArgs...)
	if where != "" {
		conditions = append(conditions, where)
	}
	const rank = "md5($1::text || ':' || m.id)"
	if position.ID != "" {
		args = append(args, position.Rank, position.ID)
		conditions = append(conditions, fmt.Sprintf("(%s,m.id)>($%d,$%d)", rank, len(args)-1, len(args)))
	}
	args = append(args, limit+1)
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT m.id,m.original_name,m.format,m.width,m.height,m.duration,m.size,m.uploaded_at,
 %s AS random_rank FROM media m WHERE %s ORDER BY random_rank,m.id LIMIT $%d`, rank, strings.Join(conditions, " AND "), len(args)), args...)
	if err != nil {
		return nil, "", err
	}
	return collectMediaPage(rows, limit, func(row pgx.Row, m *db.Media) (randomCursor, error) {
		var rank string
		err := row.Scan(&m.ID, &m.OriginalName, &m.Format, &m.Width, &m.Height, &m.Duration, &m.Size, &m.UploadedAt, &rank)
		return randomCursor{Seed: seed, Rank: rank, ID: m.ID}, err
	})
}
