package search

import (
	"context"
	"sort"

	"era/booru/ent"
	"era/booru/ent/media"
	"era/booru/ent/mediavector"
	"era/booru/ent/vector"

	"entgo.io/ent/dialect/sql"
	pgvector "github.com/pgvector/pgvector-go"
)

// SimilarMediaByVector returns media ordered by similarity to the provided
// vector. When Bleve vector search is available the build can provide a
// specialised implementation via build tags. The default implementation uses
// pgvector for similarity calculation.
func SimilarMediaByVector(
	ctx context.Context,
	db *ent.Client,
	vectorName string,
	query []float32,
	limit, offset int,
	excludeID string,
	includeIDs []string,
) ([]*ent.Media, int, error) {
	if limit <= 0 || len(query) == 0 {
		return []*ent.Media{}, 0, nil
	}

	vec := pgvector.NewVector(query)
	baseQuery := db.MediaVector.Query().
		Where(mediavector.HasVectorWith(vector.NameEQ(vectorName)))

	if excludeID != "" {
		baseQuery = baseQuery.Where(mediavector.MediaIDNEQ(excludeID))
	}
	if len(includeIDs) > 0 {
		baseQuery = baseQuery.Where(mediavector.MediaIDIn(includeIDs...))
	}

	total, err := baseQuery.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*ent.Media{}, 0, nil
	}

	mvQuery := baseQuery.Clone()

	mvQuery = mvQuery.Order(func(s *sql.Selector) {
		s.OrderExpr(sql.ExprFunc(func(b *sql.Builder) {
			b.WriteString(mediavector.Table)
			b.WriteByte('.')
			b.WriteString(mediavector.FieldValue)
			b.WriteString(" <#> ")
			b.Arg(vec)
		}))
	}).Limit(limit)

	if offset > 0 {
		mvQuery = mvQuery.Offset(offset)
	}

	ids, err := mvQuery.Select(mediavector.FieldMediaID).Strings(ctx)
	if err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return []*ent.Media{}, total, nil
	}

	medias, err := db.Media.Query().
		Where(media.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	if len(medias) <= 1 {
		return medias, total, nil
	}

	order := make(map[string]int, len(ids))
	for idx, id := range ids {
		order[id] = idx
	}

	sort.SliceStable(medias, func(i, j int) bool {
		return order[medias[i].ID] < order[medias[j].ID]
	})

	return medias, total, nil
}
