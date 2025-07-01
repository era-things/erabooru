package db

import (
	"context"

	"era/booru/ent"
	"era/booru/ent/attribute"
)

var UploadDatePropertyID int

// InitDefaultProperties ensures built-in properties exist and caches their IDs.
func InitDefaultProperties(ctx context.Context, db *ent.Client) error {
	prop, err := FindOrCreateProperty(ctx, db, "Upload Date", attribute.TypeDate)
	if err != nil {
		return err
	}
	UploadDatePropertyID = prop.ID
	return nil
}
