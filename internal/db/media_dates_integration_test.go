package db

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"era/booru/internal/config"
)

func TestUploadDateOverrideWaitsForIngestion(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to an isolated test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := Open(ctx, &config.Config{PostgresDSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	id := fmt.Sprintf("date-race-%d", time.Now().UnixNano())
	defer pool.Exec(context.Background(), `DELETE FROM media WHERE id=$1`, id)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InsertQueuedMedia(ctx, tx, id, "generation", "date.jpg", 100, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	// Reproduce the worker holding the media lock while completing ingestion.
	ingestion, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer ingestion.Rollback(context.Background())
	if err := LockMediaTx(ctx, ingestion, id); err != nil {
		t.Fatal(err)
	}
	if _, err := ingestion.Exec(ctx, `UPDATE media SET status='processing' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	expected := time.Date(2025, 10, 3, 0, 0, 0, 0, time.UTC)
	finished := make(chan error, 1)
	go func() { finished <- SetMediaDates(ctx, pool, id, []DateValue{{Name: "upload", Value: expected}}) }()
	// Inspect the actual database wait, rather than assuming a scheduling delay.
	for {
		var query string
		err := pool.QueryRow(ctx, `SELECT query FROM pg_stat_activity
WHERE datname=current_database() AND wait_event_type='Lock' AND pid <> pg_backend_pid()
AND (query LIKE 'SELECT pg_advisory_xact_lock%' OR query LIKE 'UPDATE media SET uploaded_at%') LIMIT 1`).Scan(&query)
		if err == nil {
			if !strings.HasPrefix(query, "SELECT pg_advisory_xact_lock") {
				t.Fatalf("date update acquired date rows before the media lock: %s", query)
			}
			break
		}
		select {
		case err := <-finished:
			t.Fatalf("date update did not wait for ingestion: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err := CompleteIngestionTx(ctx, ingestion, Media{ID: id, Generation: "generation", Format: "jpg", Width: 10, Height: 10, Size: 100}); err != nil {
		t.Fatal(err)
	}
	if err := ingestion.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	var uploaded, date time.Time
	if err := pool.QueryRow(ctx, `SELECT m.uploaded_at,d.value FROM media m JOIN media_date d ON d.media_id=m.id
WHERE m.id=$1 AND d.name='upload'`, id).Scan(&uploaded, &date); err != nil {
		t.Fatal(err)
	}
	if !uploaded.Equal(expected) || !date.Equal(expected) {
		t.Fatalf("date override lost: uploaded=%v date=%v, want %v", uploaded, date, expected)
	}
}
