package db

import (
	"context"

	"era/booru/ent"
	"era/booru/ent/vector"

	pgvector "github.com/pgvector/pgvector-go"
)

func FindOrCreateVector(ctx context.Context, db *ent.Client, name string) (*ent.Vector, error) {
	vt, err := db.Vector.Query().Where(vector.NameEQ(name)).Only(ctx)
	if ent.IsNotFound(err) {
		vt, err = db.Vector.Create().SetName(name).Save(ctx)
	}
	return vt, err
}

// VectorValue represents a vector name/value pair for SetMediaVectors.
type VectorValue struct {
	Name  string
	Value pgvector.Vector
}

// SetMediaVectors replaces all vectors on the given media item with the provided list.
func SetMediaVectors(ctx context.Context, db *ent.Client, mediaID string, vectors []VectorValue) error {
	if _, err := db.Media.UpdateOneID(mediaID).ClearVectors().Save(ctx); err != nil {
		return err
	}
	for _, d := range vectors {
		vt, err := FindOrCreateVector(ctx, db, d.Name)
		if err != nil {
			return err
		}
		if _, err := db.MediaVector.Create().SetMediaID(mediaID).SetVectorID(vt.ID).SetValue(d.Value).Save(ctx); err != nil {
			return err
		}
	}
	return nil
}
