package db

import (
	"context"
	"time"

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

// DateValue represents a date name/value pair for SetMediaDates.
type DateValue struct {
	Name  string
	Value time.Time
}

// SetMediaDates replaces all dates on the given media item with the provided list.
func SetMediaDates(ctx context.Context, db *ent.Client, mediaID string, dates []DateValue) error {
	if _, err := db.Media.UpdateOneID(mediaID).ClearDates().Save(ctx); err != nil {
		return err
	}
	for _, d := range dates {
		dt, err := FindOrCreateDate(ctx, db, d.Name)
		if err != nil {
			return err
		}
		if _, err := db.MediaDate.Create().SetMediaID(mediaID).SetDateID(dt.ID).SetValue(d.Value).Save(ctx); err != nil {
			return err
		}
	}
	return nil
}
