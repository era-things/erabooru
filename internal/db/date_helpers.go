package db

import (
	"context"

	"era/booru/ent"
	"era/booru/ent/date"
)

func FindOrCreateDate(ctx context.Context, db *ent.Client, name string) (*ent.Date, error) {
	dt, err := db.Date.Query().Where(date.NameEQ(name)).Only(ctx)
	if ent.IsNotFound(err) {
		dt, err = db.Date.Create().SetName(name).Save(ctx)
	}
	return dt, err
}
