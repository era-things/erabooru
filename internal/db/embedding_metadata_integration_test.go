package db

import (
	"context"
	"testing"
)

func TestEmbeddingVectorMetadata(t *testing.T) {
	pool := tagTestPool(t)
	ctx := context.Background()
	id := tagTestMedia(t, pool, "vector-metadata", nil)
	check := func(wantNorm *float64, wantDimensions *int) {
		t.Helper()
		var norm *float64
		var dimensions *int
		if err := pool.QueryRow(ctx, `SELECT value_norm,value_dimensions FROM media_embedding WHERE media_id=$1 AND space_id=$2`, id, DefaultEmbeddingSpace).Scan(&norm, &dimensions); err != nil {
			t.Fatal(err)
		}
		if wantNorm == nil {
			if norm != nil || dimensions != nil {
				t.Fatalf("pending embedding metadata: norm=%v, dimensions=%v", norm, dimensions)
			}
			return
		}
		if norm == nil || dimensions == nil || *norm != *wantNorm || *dimensions != *wantDimensions {
			t.Fatalf("incorrect vector metadata: norm=%v, dimensions=%v", norm, dimensions)
		}
	}
	check(nil, nil)
	dimensions := 768
	for _, example := range []struct {
		x, y float32
		norm float64
	}{{3, 4, 5}, {0, 2, 2}, {0, 0, 0}} {
		if _, err := pool.Exec(ctx, `UPDATE media_embedding SET value=$2 WHERE media_id=$1`, id, tagTestVector(example.x, example.y)); err != nil {
			t.Fatal(err)
		}
		check(&example.norm, &dimensions)
	}
	if _, err := pool.Exec(ctx, `UPDATE media_embedding SET value=NULL WHERE media_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	check(nil, nil)
}
