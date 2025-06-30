package db

import (
	"context"
	"fmt"

	"era/booru/ent"
	"era/booru/ent/attribute"
)

func FindOrCreateTag(ctx context.Context, db *ent.Client, name string) (*ent.Attribute, error) {
	tg, err := db.Attribute.Query().Where(attribute.NameEQ(name)).Only(ctx)
	if ent.IsNotFound(err) {
		tg, err = db.Attribute.Create().SetName(name).SetType(attribute.TypeTag).Save(ctx)
	}
	return tg, err
}

// Alternative: if you want to handle multiple tags at once
func FindOrCreateTags(ctx context.Context, db *ent.Client, tagNames []string) ([]int, error) {
	tagIDs := make([]int, 0, len(tagNames))
	for _, name := range tagNames {
		tg, err := FindOrCreateTag(ctx, db, name)
		if err != nil {
			return nil, fmt.Errorf("tag lookup/create %s: %w", name, err)
		}
		tagIDs = append(tagIDs, tg.ID)
	}
	return tagIDs, nil
}
