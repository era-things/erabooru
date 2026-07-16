package common

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
		pool, err := pgxpool.New(context.Background(), dsn)
		if err == nil {
			if err = pool.Ping(context.Background()); err == nil {
				pool.Close()
				return
			}
			pool.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("Postgres not ready after %s", timeout)
}
