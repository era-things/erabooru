package common

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"strings"

	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/minio"
	"era/booru/internal/processing"
	"era/booru/internal/queue"
	qworkers "era/booru/internal/queue/workers"

	mc "github.com/minio/minio-go/v7"
)

// ParseExport decompresses the provided gzip data and decodes the exported items.
func ParseExport(data []byte) ([]map[string]any, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(gz)
	var meta map[string]any
	if err := dec.Decode(&meta); err != nil {
		return nil, err
	}
	var items []map[string]any
	for {
		var item map[string]any
		if err := dec.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// WaitForPostgres waits until the Postgres server at the given DSN is ready or the timeout expires.
func WaitForPostgres(dsn string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		db, err := sql.Open("postgres", dsn)
		if err == nil {
			if err = db.Ping(); err == nil {
				db.Close()
				return nil
			}
			db.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("Postgres not ready after %s", timeout)
}

// WaitForMedia polls the media endpoint until the media item becomes available or the timeout expires.
func WaitForMedia(client *http.Client, baseURL, id string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("%s/api/media/%s", strings.TrimRight(baseURL, "/"), id)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(600 * time.Millisecond)
	}
	return fmt.Errorf("media %s not ingested", id)
}

// UploadAndWait uploads the given file to MinIO and waits until it is processed.
func UploadAndWait(ctx context.Context, m *minio.Client, client *http.Client, baseURL, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash, err := processing.HashFile128Hex(f)
	if err != nil {
		return "", err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return "", err
	}
	if _, err := m.PutObject(ctx, m.Bucket, hash, f, -1, mc.PutObjectOptions{ContentType: "image/png"}); err != nil {
		return "", err
	}
	if err := WaitForMedia(client, baseURL, hash, 10*time.Second); err != nil {
		return "", err
	}
	return hash, nil
}

// AddTags calls the API to add tags to the given media item.
func AddTags(client *http.Client, baseURL, id string, tags []string) error {
	b, _ := json.Marshal(struct {
		Tags []string `json:"tags"`
	}{Tags: tags})
	resp, err := client.Post(fmt.Sprintf("%s/api/media/%s/tags", strings.TrimRight(baseURL, "/"), id), "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tag response %d", resp.StatusCode)
	}
	return nil
}

// SetUploadDate sets the upload date for the given media item via the API.
func SetUploadDate(client *http.Client, baseURL, id, val string) error {
	b, _ := json.Marshal(struct {
		Dates []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"dates"`
	}{Dates: []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{{Name: "upload", Value: val}}})
	resp, err := client.Post(fmt.Sprintf("%s/api/media/%s/dates", strings.TrimRight(baseURL, "/"), id), "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("date response %d", resp.StatusCode)
	}
	return nil
}

// StartMediaWorker boots a media worker suitable for tests and returns a wrapper to stop it.
func StartMediaWorker(ctx context.Context, cfg *config.Config) (*MediaWorkerWrapper, error) {
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return nil, err
	}

	workers := river.NewWorkers()
	client, err := queue.NewClient(ctx, pool, workers, queue.ClientTypeMediaWorker)
	if err != nil {
		return nil, err
	}

	database, err := db.New(cfg, client)
	if err != nil {
		return nil, err
	}

	m, err := minio.New(cfg)
	if err != nil {
		return nil, err
	}

	river.AddWorker(workers, &qworkers.ProcessWorker{
		Minio: m,
		DB:    database,
		Cfg:   cfg,
	})

	river.AddWorker(workers, &qworkers.IndexWorker{DB: database})

	if err := client.Start(ctx); err != nil {
		return nil, err
	}

	return &MediaWorkerWrapper{
		client: client,
		pool:   pool,
		ctx:    ctx,
	}, nil
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
