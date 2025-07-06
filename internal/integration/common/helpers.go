package common

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/minio"
	"era/booru/internal/queue"
	qworkers "era/booru/internal/queue/workers"
)

// ParseExport decompresses the provided gzip data and decodes the exported items.
func ParseExport(t testing.TB, data []byte) []map[string]any {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	dec := json.NewDecoder(gz)
	var meta map[string]any
	if err := dec.Decode(&meta); err != nil {
		t.Fatalf("meta: %v", err)
	}
	var items []map[string]any
	for {
		var item map[string]any
		if err := dec.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode: %v", err)
		}
		items = append(items, item)
	}
	return items
}

// WaitForPostgres waits until the Postgres server at the given DSN is ready or the timeout expires.
func WaitForPostgres(t testing.TB, dsn string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		db, err := sql.Open("postgres", dsn)
		if err == nil {
			if err = db.Ping(); err == nil {
				db.Close()
				return
			}
			db.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("Postgres not ready after %s", timeout)
}

// StartMediaWorker boots a media worker suitable for tests and returns a wrapper to stop it.
func StartMediaWorker(t testing.TB, ctx context.Context, cfg *config.Config) *MediaWorkerWrapper {
	t.Helper()
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}

	workers := river.NewWorkers()
	client, err := queue.NewClient(ctx, pool, workers, queue.ClientTypeMediaWorker)
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	database, err := db.New(cfg, client)
	if err != nil {
		t.Fatalf("db: %v", err)
	}

	m, err := minio.New(cfg)
	if err != nil {
		t.Fatalf("minio: %v", err)
	}

	river.AddWorker(workers, &qworkers.ProcessWorker{
		Minio: m,
		DB:    database,
		Cfg:   cfg,
	})
	river.AddWorker(workers, &qworkers.IndexWorker{DB: database})

	if err := client.Start(ctx); err != nil {
		t.Fatalf("river start: %v", err)
	}

	return &MediaWorkerWrapper{client: client, pool: pool, ctx: ctx}
}

// MediaWorkerWrapper is returned by StartMediaWorker and provides a Stop method.
type MediaWorkerWrapper struct {
	client *river.Client[pgx.Tx]
	pool   *pgxpool.Pool
	ctx    context.Context
}

// Stop stops the underlying river client and closes the pool.
func (w *MediaWorkerWrapper) Stop() {
	if w.client != nil {
		w.client.Stop(context.Background())
	}
	if w.pool != nil {
		w.pool.Close()
	}
}
