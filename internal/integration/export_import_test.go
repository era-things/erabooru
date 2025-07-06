package integration_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/testcontainers/testcontainers-go"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/wait"

	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/minio"
	"era/booru/internal/processing"
	"era/booru/internal/queue"
	qworkers "era/booru/internal/queue/workers"
	"era/booru/internal/server"

	"time"

	mc "github.com/minio/minio-go/v7"
)

func parseExport(data []byte) ([]map[string]any, error) {
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

func waitForPostgres(dsn string, timeout time.Duration) error {
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

func TestExportImportCycle(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("integration test; set RUN_INTEGRATION_TESTS=1 to run")
	}

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "booru",
			"POSTGRES_USER":     "booru",
			"POSTGRES_PASSWORD": "booru",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}
	pgC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	defer pgC.Terminate(ctx)

	// Build DSN manually
	host, err := pgC.Host(ctx)
	if err != nil {
		t.Fatalf("get host: %v", err)
	}
	port, err := pgC.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("get port: %v", err)
	}
	dsn := fmt.Sprintf("postgres://booru:booru@%s:%s/booru?sslmode=disable", host, port.Port())

	mC, err := tcminio.Run(ctx, "minio/minio:RELEASE.2024-01-16T16-07-38Z",
		tcminio.WithUsername("minioadmin"), tcminio.WithPassword("minio123"))
	if err != nil {
		t.Fatalf("start minio: %v", err)
	}
	defer mC.Terminate(ctx)

	waitForPostgres(dsn, 30*time.Second)

	minioAddr, err := mC.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("minio addr: %v", err)
	}

	os.Setenv("POSTGRES_DSN", dsn)
	os.Setenv("MINIO_ROOT_USER", "minioadmin")
	os.Setenv("MINIO_ROOT_PASSWORD", "minio123")
	os.Setenv("MINIO_BUCKET", "boorubucket")
	os.Setenv("MINIO_PREVIEW_BUCKET", "previews")
	os.Setenv("MINIO_INTERNAL_ENDPOINT", minioAddr)
	os.Setenv("MINIO_PUBLIC_HOST", "")
	os.Setenv("MINIO_PUBLIC_PREFIX", "boorubucket")
	os.Setenv("MINIO_SSL", "false")
	os.Setenv("VIDEO_WORKER_URL", "http://invalid")
	os.Setenv("DEV_MODE", "true")
	bleveDir := filepath.Join(t.TempDir(), "bleve")
	os.Setenv("BLEVE_PATH", bleveDir)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	srv, err := server.New(ctx, cfg)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	defer srv.Close()

	// Start the media worker
	mediaWorker, err := startMediaWorker(ctx, cfg)
	if err != nil {
		t.Fatalf("media worker: %v", err)
	}
	defer mediaWorker.Stop()

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()
	client := ts.Client()

	waitFor := func(id string) {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			resp, err := client.Get(ts.URL + "/api/media/" + id)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatalf("media %s not ingested", id)
	}

	upload := func(obj, path string) string {
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		defer f.Close()

		// Compute hash of the file
		hash, err := processing.HashFile128Hex(f)
		if err != nil {
			t.Fatalf("hash %s: %v", path, err)
		}

		// Reset file pointer to beginning
		if _, err := f.Seek(0, 0); err != nil {
			t.Fatalf("seek %s: %v", path, err)
		}

		// Upload with hash as object name (no extension)
		if _, err := srv.Minio.PutObject(ctx, srv.Minio.Bucket, hash, f, -1, mc.PutObjectOptions{ContentType: "image/png"}); err != nil {
			t.Fatalf("put %s: %v", hash, err)
		}

		waitFor(hash)
		return hash
	}

	img1Hash := upload("img1.png", filepath.Join("testdata", "img1.png"))
	img2Hash := upload("img2.png", filepath.Join("testdata", "img2.png"))
	img3Hash := upload("img3.png", filepath.Join("testdata", "img3.png"))

	addTags := func(id string, tags []string) {
		b, _ := json.Marshal(struct {
			Tags []string `json:"tags"`
		}{Tags: tags})
		resp, err := client.Post(ts.URL+"/api/media/"+id+"/tags", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("post tags %s: %v", id, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("tag response %d", resp.StatusCode)
		}
	}

	addTags(img1Hash, []string{"alpha"})
	addTags(img2Hash, []string{"beta"})
	addTags(img3Hash, []string{"gamma"})

	setDate := func(id, val string) {
		b, _ := json.Marshal(struct {
			Dates []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"dates"`
		}{Dates: []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}{{Name: "upload", Value: val}}})
		resp, err := client.Post(ts.URL+"/api/media/"+id+"/dates", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("post date %s: %v", id, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("date response %d", resp.StatusCode)
		}
	}

	setDate(img1Hash, "2021-01-02")
	setDate(img2Hash, "2022-02-03")

	resp, err := client.Get(ts.URL + "/api/admin/export-tags")
	if err != nil {
		t.Fatalf("export request: %v", err)
	}
	first, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export response %d", resp.StatusCode)
	}
	items1, err := parseExport(first)
	if err != nil {
		t.Fatalf("parse export: %v", err)
	}

	// purge db via container
	if code, out, err := pgC.Exec(ctx, []string{
		"psql", "-U", "booru", "-d", "booru",
		"-c", "DROP SCHEMA public CASCADE; CREATE SCHEMA public;",
	}); err != nil || code != 0 {
		t.Fatalf("reset via container Exec failed (%d): %s", code, out)
	}

	time.Sleep(2 * time.Second)

	resp, err = client.Post(ts.URL+"/api/admin/regenerate", "application/json", nil)
	if err != nil {
		t.Fatalf("regenerate request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("regenerate response %d", resp.StatusCode)
	}

	waitFor(img1Hash)
	waitFor(img2Hash)
	waitFor(img3Hash)

	resp, err = client.Post(ts.URL+"/api/admin/import-tags", "application/gzip", bytes.NewReader(first))
	if err != nil {
		t.Fatalf("import request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("import response %d", resp.StatusCode)
	}

	time.Sleep(1 * time.Second)

	resp, err = client.Get(ts.URL + "/api/admin/export-tags")
	if err != nil {
		t.Fatalf("re-export request: %v", err)
	}
	second, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("re-export response %d", resp.StatusCode)
	}
	items2, err := parseExport(second)
	if err != nil {
		t.Fatalf("parse second export: %v", err)
	}

	sort.Slice(items1, func(i, j int) bool { return items1[i]["id"].(string) < items1[j]["id"].(string) })
	sort.Slice(items2, func(i, j int) bool { return items2[i]["id"].(string) < items2[j]["id"].(string) })

	if !reflect.DeepEqual(items1, items2) {
		t.Fatalf("export/import mismatch")
	}
}

func startMediaWorker(ctx context.Context, cfg *config.Config) (*MediaWorkerWrapper, error) {
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

	// Register BOTH workers (even though media worker only processes ProcessWorker)
	river.AddWorker(workers, &qworkers.ProcessWorker{
		Minio: m,
		DB:    database,
		Cfg:   cfg,
	})

	// Add this - needed for media worker to enqueue index jobs
	river.AddWorker(workers, &qworkers.IndexWorker{
		DB: database,
	})

	if err := client.Start(ctx); err != nil {
		return nil, err
	}

	return &MediaWorkerWrapper{
		client: client,
		pool:   pool,
		ctx:    ctx,
	}, nil
}

type MediaWorkerWrapper struct {
	client *river.Client[pgx.Tx]
	pool   *pgxpool.Pool
	ctx    context.Context
}

func (w *MediaWorkerWrapper) Stop() {
	if w.client != nil {
		w.client.Stop(context.Background())
	}
	if w.pool != nil {
		w.pool.Close()
	}
}
