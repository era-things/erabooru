package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"era/booru/internal/config"
	"github.com/jackc/pgx/v5"
	pgvector "github.com/pgvector/pgvector-go"
)

func TestEmbeddingRetriesAndTerminalGuards(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to an isolated test database")
	}
	ctx := context.Background()
	pool, err := Open(ctx, &config.Config{PostgresDSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	id := fmt.Sprintf("embedding-lifecycle-%d", time.Now().UnixNano())
	defer pool.Exec(ctx, `DELETE FROM media WHERE id=$1`, id)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := InsertQueuedMedia(ctx, tx, id, "generation", "lifecycle.png", 10, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := CompleteIngestionTx(ctx, tx, Media{ID: id, Generation: "generation", Format: "png", Width: 10, Height: 10, Size: 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := QueueEmbeddingTx(ctx, tx, id, "generation", DefaultEmbeddingSpace); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	start := func(want bool) {
		t.Helper()
		got, err := StartEmbedding(ctx, pool, id, "generation", DefaultEmbeddingSpace)
		if err != nil || got != want {
			t.Fatalf("start: %v %v, want %v", got, err, want)
		}
	}
	start(true)
	if err := RecordEmbeddingError(ctx, pool, id, "generation", DefaultEmbeddingSpace, "retrying", errors.New("temporary")); err != nil {
		t.Fatal(err)
	}
	start(true)
	if err := RecordEmbeddingError(ctx, pool, id, "generation", DefaultEmbeddingSpace, "failed", errors.New("exhausted")); err != nil {
		t.Fatal(err)
	}
	start(false)
	vector := pgvector.NewVector(make([]float32, 768))
	if err := CompleteEmbedding(ctx, pool, id, "generation", DefaultEmbeddingSpace, vector); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("terminal completion accepted: %v", err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := QueueEmbeddingTx(ctx, tx, id, "generation", DefaultEmbeddingSpace); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	start(true)
	if err := CompleteEmbedding(ctx, pool, id, "obsolete", DefaultEmbeddingSpace, vector); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("obsolete completion accepted: %v", err)
	}
	if err := CompleteEmbedding(ctx, pool, id, "generation", DefaultEmbeddingSpace, vector); err != nil {
		t.Fatal(err)
	}
	if err := RecordEmbeddingError(ctx, pool, id, "generation", DefaultEmbeddingSpace, "failed", errors.New("late error")); err != nil {
		t.Fatal(err)
	}
	status, err := EmbeddingLifecycle(ctx, pool, id, DefaultEmbeddingSpace)
	if err != nil || status != "ready" {
		t.Fatalf("ready vector downgraded: %s %v", status, err)
	}
	if err := RecordIngestionError(ctx, pool, id, "generation", "failed", errors.New("late ingestion error")); err != nil {
		t.Fatal(err)
	}
	_, status, err = MediaLifecycle(ctx, pool, id)
	if err != nil || status != "ready" {
		t.Fatalf("ready media downgraded: %s %v", status, err)
	}
}
