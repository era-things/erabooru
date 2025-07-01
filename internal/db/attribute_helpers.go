package db

import (
	"context"
	"fmt"
	"time"

	"era/booru/ent"
	"era/booru/ent/attribute"
	"era/booru/ent/mediaattribute"
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

// FindOrCreateProperty looks up an attribute by name and type, creating it if needed.
func FindOrCreateProperty(ctx context.Context, db *ent.Client, name string, typ attribute.Type) (*ent.Attribute, error) {
	at, err := db.Attribute.Query().Where(attribute.NameEQ(name)).Only(ctx)
	if ent.IsNotFound(err) {
		at, err = db.Attribute.Create().SetName(name).SetType(typ).Save(ctx)
	}
	return at, err
}

// SetDateProperty stores a date property value on the media item.
func SetDateProperty(ctx context.Context, db *ent.Client, mediaID string, attrID int, t time.Time) error {
	val := t.Format("2006-01-02")
	ma, err := db.MediaAttribute.Query().
		Where(mediaattribute.MediaIDEQ(mediaID)).
		Where(mediaattribute.AttributeIDEQ(attrID)).
		Only(ctx)
	if ent.IsNotFound(err) {
		_, err = db.MediaAttribute.Create().
			SetMediaID(mediaID).
			SetAttributeID(attrID).
			SetValue(val).
			Save(ctx)
		return err
	}
	if err != nil {
		return err
	}
	_, err = ma.Update().SetValue(val).Save(ctx)
	return err
}

// GetDateProperty retrieves the date property for a media item.
func GetDateProperty(ctx context.Context, db *ent.Client, mediaID string, attrID int) (*time.Time, error) {
	ma, err := db.MediaAttribute.Query().
		Where(mediaattribute.MediaIDEQ(mediaID)).
		Where(mediaattribute.AttributeIDEQ(attrID)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if ma.Value == nil {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", *ma.Value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
