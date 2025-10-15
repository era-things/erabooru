package db

import (
	"context"

	"era/booru/ent"
	"era/booru/ent/date"
	"era/booru/ent/mediadate"

	"entgo.io/ent/dialect/sql"
)

// ListMediaByDate returns media records ordered by the specified named date.
// If includeIDs is non-nil, only media matching one of the provided IDs are returned.
func ListMediaByDate(ctx context.Context, client *ent.Client, dateName string, limit, offset int, includeIDs []string) ([]*ent.Media, int, error) {
	dt, err := client.Date.Query().Where(date.NameEQ(dateName)).Only(ctx)
	switch {
	case ent.IsNotFound(err):
		return []*ent.Media{}, 0, nil
	case err != nil:
		return nil, 0, err
	}

	if includeIDs != nil && len(includeIDs) == 0 {
		return []*ent.Media{}, 0, nil
	}

	baseQuery := client.MediaDate.Query().Where(mediadate.DateIDEQ(dt.ID))
	if len(includeIDs) > 0 {
		baseQuery = baseQuery.Where(mediadate.MediaIDIn(includeIDs...))
	}

	total, err := baseQuery.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*ent.Media{}, 0, nil
	}

	rowsQuery := baseQuery.Clone().Order(mediadate.ByValue(sql.OrderDesc()))

	if offset > 0 {
		rowsQuery = rowsQuery.Offset(offset)
	}
	if limit > 0 {
		rowsQuery = rowsQuery.Limit(limit)
	}

	rows, err := rowsQuery.WithMedia().All(ctx)
	if err != nil {
		return nil, 0, err
	}

	items := make([]*ent.Media, 0, len(rows))
	for _, row := range rows {
		if row.Edges.Media != nil {
			items = append(items, row.Edges.Media)
		}
	}
	return items, total, nil
}
