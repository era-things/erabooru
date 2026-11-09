package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"era/booru/internal/config"

	pgvector "github.com/pgvector/pgvector-go"
)

func TestCleanSchemaLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to run the schema integration test")
	}

	ctx := context.Background()
	mediaID := fmt.Sprintf("schema-lifecycle-%d", time.Now().UnixNano())
	pool, err := Open(ctx, &config.Config{PostgresDSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	defer pool.Exec(context.Background(), `DELETE FROM media WHERE id=$1`, mediaID)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := InsertQueuedMedia(ctx, tx, mediaID, "generation", "2026-11-09T13-04-05Z.png", 123, time.Now().UTC())
	if err != nil || !created {
		t.Fatalf("insert queued media: created=%v err=%v", created, err)
	}
	if err := CompleteIngestionTx(ctx, tx, Media{ID: mediaID, Generation: "generation", Format: "png", Width: 10, Height: 20, Size: 123}); err != nil {
		t.Fatal(err)
	}
	if _, err := QueueEmbeddingTx(ctx, tx, mediaID, "generation", DefaultEmbeddingSpace); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	started, err := StartEmbedding(ctx, pool, mediaID, "generation", DefaultEmbeddingSpace)
	if err != nil || !started {
		t.Fatalf("start embedding: started=%v err=%v", started, err)
	}
	if err := CompleteEmbedding(ctx, pool, mediaID, "generation", DefaultEmbeddingSpace, pgvector.NewVector(make([]float32, 768))); err != nil {
		t.Fatal(err)
	}
	if err := SetMediaTags(ctx, pool, mediaID, []string{"one", "two"}); err != nil {
		t.Fatal(err)
	}
	if err := SetMediaDates(ctx, pool, mediaID, []DateValue{{Name: "upload", Value: time.Now().UTC()}}); err != nil {
		t.Fatal(err)
	}

	item, err := GetMedia(ctx, pool, mediaID)
	if err != nil {
		t.Fatal(err)
	}
	if item.OriginalName != "2026-11-09T13-04-05Z.png" {
		t.Fatalf("original filename lost: %q", item.OriginalName)
	}
	if len(item.Tags) != 2 || len(item.Embeddings) != 1 || item.Embeddings[0].Status != "ready" {
		t.Fatalf("unexpected media: %+v", item)
	}
}
