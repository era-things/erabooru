package search

import (
	"context"
	"fmt"

	"era/booru/internal/db"
	"era/booru/internal/tagmodel"
	"github.com/jackc/pgx/v5"
)

// TagReviewMedia uses the same classifier and centroid fallback as tag recommendations.
// Representatives use the positive centroid alone, so they show typical tagged items.
func TagReviewMedia(ctx context.Context, pool interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, tag, profile, filter string, representatives bool, limit int, cursor string) ([]db.Media, string, error) {
	if limit <= 0 {
		return []db.Media{}, "", nil
	}
	position, err := decodeCursor[vectorCursor](cursor)
	if err != nil {
		return nil, "", err
	}
	args := []any{tag, profile, representatives, tagmodel.MinimumPositives, tagmodel.Version, position.ID, position.Distance, limit + 1}
	where, filterArgs := WhereSQL(filter, len(args)+1)
	args = append(args, filterArgs...)
	if where != "" {
		where = " AND (" + where + ")"
	}
	// Keep the single selected model behind an optimization fence. Otherwise
	// PostgreSQL can score media against every tag before applying the tag name.
	// Materialize scores too, so cursor/null filtering and sorting reuse each
	// vector calculation. Tag-level norms only need to be calculated once.
	// Representatives start from the tag's membership index instead of scanning
	// all embeddings and testing membership with a correlated subquery.
	membershipJoin := ""
	membershipFilter := ` AND NOT EXISTS(SELECT FROM media_tag mt WHERE mt.media_id=m.id AND mt.tag_id=s.tag_id)
 AND NOT EXISTS(SELECT FROM media_negative_tag n WHERE n.media_id=m.id AND n.tag_id=s.tag_id)`
	if representatives {
		membershipJoin = " JOIN media_tag mt ON mt.media_id=e.media_id AND mt.tag_id=s.tag_id"
		membershipFilter = ""
	}
	score := (tagmodel.ScoreSQL{
		Value: "e.value", ValueNorm: "e.value_norm",
		Positive: "s.positive_sum", PositiveNorm: "s.positive_norm",
		Negative: "s.negative_sum", NegativeNorm: "s.negative_norm", NegativeCount: "s.negative_count",
		Classifier: "s.positive_count >= $4 AND s.weights IS NOT NULL AND s.model_version=$5", Weights: "s.weights", Bias: "s.bias",
	}).Expression()
	rows, err := pool.Query(ctx, `WITH model AS MATERIALIZED (
 SELECT s.*,c.weights,c.bias,c.model_version,
 vector_norm(s.positive_sum) AS positive_norm,vector_norm(s.negative_sum) AS negative_norm
 FROM tag t JOIN tag_statistics s ON s.tag_id=t.id
 LEFT JOIN tag_classifier c ON c.tag_id=s.tag_id AND c.space_id=s.space_id AND c.profile=s.profile
 WHERE t.name=$1 AND t.name<>'tagme' AND s.space_id='vision' AND s.profile=$2 AND s.profile<>''
 AND s.positive_count>0
 ), scored AS MATERIALIZED (
 SELECT m.id,m.original_name,m.format,m.width,m.height,
 CASE WHEN $3::boolean THEN CASE WHEN s.positive_norm>0 THEN 1-(e.value <=> s.positive_sum) END
 ELSE `+score+` END AS score
 FROM model s
 JOIN media_embedding e ON e.space_id=s.space_id AND e.profile=s.profile
	JOIN media m ON m.id=e.media_id`+membershipJoin+`
 WHERE e.status='ready' AND m.status='ready' AND e.value_dimensions=768 AND e.value_norm>0`+membershipFilter+where+`
 ) SELECT id,original_name,format,width,height,-score AS distance FROM scored
 WHERE score IS NOT NULL AND ($6='' OR (-score,id)>($7,$6))
 ORDER BY distance,id LIMIT $8`, args...)
	if err != nil {
		return nil, "", fmt.Errorf("tag review: %w", err)
	}
	return collectMediaPage(rows, limit, func(row pgx.Row, m *db.Media) (vectorCursor, error) {
		var distance float64
		err := row.Scan(&m.ID, &m.OriginalName, &m.Format, &m.Width, &m.Height, &distance)
		return vectorCursor{Distance: distance, ID: m.ID}, err
	})
}
