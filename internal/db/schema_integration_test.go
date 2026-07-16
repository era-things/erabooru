package db

import (
	"context"
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
	pool, err := Open(ctx, &config.Config{PostgresDSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := InsertQueuedMedia(ctx, tx, "media", "generation", 123, time.Now().UTC())
	if err != nil || !created {
		t.Fatalf("insert queued media: created=%v err=%v", created, err)
	}
	if err := CompleteIngestionTx(ctx, tx, Media{ID: "media", Generation: "generation", Format: "png", Width: 10, Height: 20, Size: 123}); err != nil {
		t.Fatal(err)
	}
	if err := QueueEmbeddingTx(ctx, tx, "media", "generation", DefaultEmbeddingSpace); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	started, err := StartEmbedding(ctx, pool, "media", "generation", DefaultEmbeddingSpace)
	if err != nil || !started {
		t.Fatalf("start embedding: started=%v err=%v", started, err)
	}
	if err := CompleteEmbedding(ctx, pool, "media", "generation", DefaultEmbeddingSpace, pgvector.NewVector(make([]float32, 768))); err != nil {
		t.Fatal(err)
	}
	if err := SetMediaTags(ctx, pool, "media", []string{"one", "two"}); err != nil {
		t.Fatal(err)
	}
	if err := SetMediaDates(ctx, pool, "media", []DateValue{{Name: "upload", Value: time.Now().UTC()}}); err != nil {
		t.Fatal(err)
	}

	item, err := GetMedia(ctx, pool, "media")
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Tags) != 2 || len(item.Embeddings) != 1 || item.Embeddings[0].Status != "ready" {
		t.Fatalf("unexpected media: %+v", item)
	}
}
