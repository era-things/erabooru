package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"era/booru/internal/config"
	"era/booru/internal/db"
	embed "era/booru/internal/embeddings"
	"era/booru/internal/processing"
	"era/booru/internal/queue"
	"era/booru/internal/storage"
	"era/booru/internal/workers/mediaworker"
	"github.com/gin-gonic/gin"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

func TestMediaImportLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to an isolated test database")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, &config.Config{PostgresDSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worker := &mediaworker.ProcessWorker{Store: store, DB: pool, Decode: processing.NewLimiter(1)}
	workers := river.NewWorkers()
	river.AddWorker(workers, worker)
	client, err := queue.NewClient(ctx, pool, workers, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	profile := embed.Profile{ID: "test-profile", Dimensions: 768}
	router := gin.New()
	router.POST("/api/media/import", importMediaHandler(pool, store, client, profile))
	router.POST("/api/media", uploadMediaHandler(pool, store, client))
	file, err := os.ReadFile("../integration/testdata/img1.png")
	if err != nil {
		t.Fatal(err)
	}
	vector := make([]float32, 768)
	vector[0] = 2
	metadata, _ := json.Marshal(map[string]any{"profile": profile.ID, "vector": vector})
	upload := func(metadata []byte, extra bool) *httptest.ResponseRecorder {
		req, _ := importRequest(t, string(metadata), false, extra, file)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	// Extra parts must not create database records or leave originals behind.
	if response := upload(metadata, true); response.Code != 400 {
		t.Fatalf("extra part accepted: %d %s", response.Code, response.Body)
	}
	count := 0
	if err := store.WalkOriginals(func(string) error { count++; return nil }); err != nil || count != 0 {
		t.Fatal("invalid import left a file", err)
	}
	response := upload(metadata, false)
	if response.Code != 201 {
		t.Fatalf("import: %d %s", response.Code, response.Body)
	}
	var result struct {
		ID      string `json:"id"`
		Existed bool   `json:"existed"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "DELETE FROM media WHERE id=$1", result.ID)
	var generation, mediaStatus, embeddingStatus, originalName string
	var stored pgvector.Vector
	if err := pool.QueryRow(ctx, `SELECT m.generation,m.status,e.status,e.value,m.original_name FROM media m JOIN media_embedding e ON e.media_id=m.id WHERE m.id=$1`, result.ID).Scan(&generation, &mediaStatus, &embeddingStatus, &stored, &originalName); err != nil {
		t.Fatal(err)
	}
	if originalName != "sample.mp4" {
		t.Fatalf("import filename lost: %q", originalName)
	}
	if mediaStatus != "queued" || embeddingStatus != "ready" || stored.Slice()[0] != 1 {
		t.Fatal("import was not atomic / normalized")
	}
	// Rejecting a malformed duplicate must leave the accepted original untouched.
	if response := upload(metadata, true); response.Code != 400 {
		t.Fatalf("malformed duplicate accepted: %d", response.Code)
	}
	if original, err := os.ReadFile(store.OriginalPath(result.ID)); err != nil || !bytes.Equal(original, file) {
		t.Fatalf("malformed duplicate damaged original: %v", err)
	}
	// Metadata ingestion must preserve the vector and never schedule inference.
	if err := worker.Work(ctx, &river.Job[queue.ProcessArgs]{JobRow: &rivertype.JobRow{ID: 1}, Args: queue.ProcessArgs{Key: result.ID, Generation: generation, ContentType: "image/png"}}); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	queued, err := db.QueueEmbeddingTx(ctx, tx, result.ID, generation, db.DefaultEmbeddingSpace)
	if err != nil || queued {
		t.Fatalf("imported vector requeued: %t %v", queued, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	started, err := db.StartEmbedding(ctx, pool, result.ID, generation, db.DefaultEmbeddingSpace)
	if err != nil || started {
		t.Fatal("stale inference job can overwrite imported vector", err)
	}
	item, err := db.GetMedia(ctx, pool, result.ID)
	if err != nil || item.OriginalName != "sample.mp4" || len(item.Embeddings) != 1 || item.Embeddings[0].Status != "ready" {
		t.Fatal("import not searchable after processing", err)
	}
	var embedJobs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM river_job WHERE kind='embed_media' AND args->>'key'=$1`, result.ID).Scan(&embedJobs); err != nil || embedJobs != 0 {
		t.Fatal("inference job created", err)
	}
	vector[0] = 0
	vector[1] = 1
	metadata, _ = json.Marshal(map[string]any{"profile": profile.ID, "vector": vector})
	if response := upload(metadata, false); response.Code != 200 {
		t.Fatalf("duplicate: %d %s", response.Code, response.Body)
	}
	if err := pool.QueryRow(ctx, `SELECT value FROM media_embedding WHERE media_id=$1`, result.ID).Scan(&stored); err != nil || stored.Slice()[0] != 1 || stored.Slice()[1] != 0 {
		t.Fatal("duplicate overwrote vector", err)
	}
	// The ordinary API upload path still queues metadata without importing a vector.
	ordinary, err := os.ReadFile("../integration/testdata/img2.png")
	if err != nil {
		t.Fatal(err)
	}
	request := mediaUploadRequest(t, "2026-11-09T13-04-05Z — обычный.png", "image/png", ordinary, false)
	normalResponse := httptest.NewRecorder()
	router.ServeHTTP(normalResponse, request)
	if normalResponse.Code != 201 {
		t.Fatalf("ordinary upload: %d %s", normalResponse.Code, normalResponse.Body)
	}
	var normal struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(normalResponse.Body.Bytes(), &normal); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "DELETE FROM media WHERE id=$1", normal.ID)
	var ordinaryName string
	if err := pool.QueryRow(ctx, "SELECT original_name FROM media WHERE id=$1", normal.ID).Scan(&ordinaryName); err != nil || ordinaryName != "2026-11-09T13-04-05Z — обычный.png" {
		t.Fatalf("ordinary filename lost: %q %v", ordinaryName, err)
	}
	duplicate := httptest.NewRecorder()
	router.ServeHTTP(duplicate, mediaUploadRequest(t, "renamed.png", "image/png", ordinary, false))
	if duplicate.Code != 200 {
		t.Fatalf("duplicate: %d", duplicate.Code)
	}
	if err := pool.QueryRow(ctx, "SELECT original_name FROM media WHERE id=$1", normal.ID).Scan(&ordinaryName); err != nil || ordinaryName != "2026-11-09T13-04-05Z — обычный.png" {
		t.Fatalf("duplicate replaced original name: %q %v", ordinaryName, err)
	}
	var imported int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM media_embedding WHERE media_id=$1", normal.ID).Scan(&imported); err != nil || imported != 0 {
		t.Fatal("ordinary upload imported a vector", err)
	}

}
