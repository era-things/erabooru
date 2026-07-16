package search

import (
	"context"
	"fmt"

	"era/booru/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

func SimilarMediaByVector(ctx context.Context, pool *pgxpool.Pool, vectorName string, query []float32, limit, offset int, excludeID, filterExpr string) ([]db.Media, int, error) {
	if limit <= 0 || len(query) == 0 {
		return []db.Media{}, 0, nil
	}
	args := []any{vectorName, pgvector.NewVector(query)}
	conditions := []string{"mv.name = $1", "mv.value <#> $2 IS NOT NULL"}
	if excludeID != "" {
		args = append(args, excludeID)
		conditions = append(conditions, fmt.Sprintf("m.id <> $%d", len(args)))
	}
	where, filterArgs := WhereSQL(filterExpr, len(args)+1)
	args = append(args, filterArgs...)
	if where != "" {
		conditions = append(conditions, where)
	}
	conditionSQL := " WHERE " + joinConditions(conditions)
	var total int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM media_vector mv JOIN media m ON m.id = mv.media_id`+conditionSQL, args...).Scan(&total); err != nil || total == 0 {
		return nil, total, err
	}
	args = append(args, limit, offset)
	querySQL := fmt.Sprintf(`SELECT m.id, m.format, m.width, m.height, m.duration, m.size, m.uploaded_at
FROM media_vector mv JOIN media m ON m.id = mv.media_id%s
ORDER BY mv.value <#> $2 LIMIT $%d OFFSET $%d`, conditionSQL, len(args)-1, len(args))
	rows, err := pool.Query(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, err
	}
	items, err := scanMedia(rows)
	return items, total, err
}

func joinConditions(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += " AND "
		}
		result += part
	}
	return result
}
