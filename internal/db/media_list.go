package db

import (
	"context"

	"era/booru/ent"
	"era/booru/ent/date"
	"era/booru/ent/mediadate"

	"entgo.io/ent/dialect/sql"
)

// ListMediaByDate returns media records ordered by the specified named date.
func ListMediaByDate(ctx context.Context, client *ent.Client, dateName string, limit, offset int) ([]*ent.Media, int, error) {
	dt, err := client.Date.Query().Where(date.NameEQ(dateName)).Only(ctx)
	switch {
	case ent.IsNotFound(err):
		return []*ent.Media{}, 0, nil
	case err != nil:
		return nil, 0, err
	}

	total, err := client.MediaDate.Query().Where(mediadate.DateIDEQ(dt.ID)).Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*ent.Media{}, 0, nil
	}

	query := client.MediaDate.Query().
		Where(mediadate.DateIDEQ(dt.ID)).
		Order(mediadate.ByValue(sql.OrderDesc()))

	if offset > 0 {
		query = query.Offset(offset)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	rows, err := query.WithMedia().All(ctx)
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
