package search

import (
	"context"
	"fmt"

	"era/booru/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

func SimilarMediaByVector(ctx context.Context, pool *pgxpool.Pool, vectorName string, query []float32, limit int, cursor, excludeID, filterExpr string) ([]db.Media, string, error) {
	if limit <= 0 || len(query) == 0 {
		return []db.Media{}, "", nil
	}
	args := []any{vectorName, pgvector.NewVector(query)}
	conditions := []string{"me.space_id = $1", "me.status = 'ready'", "m.status = 'ready'", "me.value IS NOT NULL"}
	if excludeID != "" {
		args = append(args, excludeID)
		conditions = append(conditions, fmt.Sprintf("m.id <> $%d", len(args)))
	}
	where, filterArgs := WhereSQL(filterExpr, len(args)+1)
	args = append(args, filterArgs...)
	if where != "" {
		conditions = append(conditions, where)
	}
	position, err := decodeCursor[vectorCursor](cursor)
	if err != nil {
		return nil, "", err
	}
	if position.ID != "" {
		args = append(args, position.Distance, position.ID)
		conditions = append(conditions, fmt.Sprintf("(me.value::vector(768) <#> $2, m.id) > ($%d, $%d)", len(args)-1, len(args)))
	}
	args = append(args, limit+1)
	conditionSQL := " WHERE " + joinConditions(conditions)
	querySQL := fmt.Sprintf(`SELECT m.id, m.format, m.width, m.height, m.duration, m.size, m.uploaded_at
	, me.value::vector(768) <#> $2 AS distance
FROM media_embedding me JOIN media m ON m.id = me.media_id%s
ORDER BY distance, m.id LIMIT $%d`, conditionSQL, len(args))
	rows, err := pool.Query(ctx, querySQL, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := []db.Media{}
	distances := []float64{}
	for rows.Next() {
		var item db.Media
		var distance float64
		if err := rows.Scan(&item.ID, &item.Format, &item.Width, &item.Height, &item.Duration, &item.Size, &item.UploadedAt, &distance); err != nil {
			return nil, "", err
		}
		items = append(items, item)
		distances = append(distances, distance)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		distances = distances[:limit]
		next = encodeCursor(vectorCursor{Distance: distances[len(distances)-1], ID: items[len(items)-1].ID})
	}
	return items, next, nil
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
