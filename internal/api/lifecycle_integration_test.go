package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/processing"
	"era/booru/internal/queue"
	"era/booru/internal/storage"
	"era/booru/internal/workers/mediaworker"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

func TestLifecycleRetryDeletionAndReupload(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to an isolated test database")
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
	process := &mediaworker.ProcessWorker{Store: store, DB: pool, Decode: processing.NewLimiter(1)}
	cleanup := &mediaworker.CleanupWorker{Store: store, DB: pool}
	workers := river.NewWorkers()
	river.AddWorker(workers, process)
	river.AddWorker(workers, cleanup)
	client, err := queue.NewClient(ctx, pool, workers, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterMediaRoutes(router, pool, store, client, processing.NewInferenceLimiter(1))
	RegisterAdminRoutes(router, pool, store, client)
	request := func(method, path string, body []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "image/png")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	// Deliberately invalid bytes let ingestion fail without depending on ffmpeg.
	content := []byte(fmt.Sprintf("lifecycle integration: invalid image %d", time.Now().UnixNano()))
	upload := request(http.MethodPost, "/api/media", content)
	if upload.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", upload.Code, upload.Body)
	}
	var uploaded struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(upload.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}
	id := uploaded.ID
	defer pool.Exec(context.Background(), `DELETE FROM river_job WHERE args->>'key'=$1`, id)
	defer pool.Exec(context.Background(), `DELETE FROM media WHERE id=$1`, id)
	defer pool.Exec(context.Background(), `DELETE FROM deleted_media WHERE id=$1`, id)
	generation, status, err := db.MediaLifecycle(ctx, pool, id)
	if err != nil || status != "queued" {
		t.Fatalf("queued: %s %v", status, err)
	}
	detail := request(http.MethodGet, "/api/media/"+id, nil)
	if detail.Code != 200 {
		t.Fatalf("pending detail: %d %s", detail.Code, detail.Body)
	}
	var state struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(detail.Body.Bytes(), &state); err != nil || state.Status != "queued" {
		t.Fatalf("pending response: %s %v", detail.Body, err)
	}
	// Probe failure is retryable whether ffprobe is missing or rejects these bytes.
	job := &river.Job[queue.ProcessArgs]{JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 3}, Args: queue.ProcessArgs{Key: id, Generation: generation, ContentType: "video/mp4"}}
	if err := process.Work(ctx, job); err == nil {
		t.Fatal("invalid video accepted")
	}
	_, status, err = db.MediaLifecycle(ctx, pool, id)
	if err != nil || status != "retrying" {
		t.Fatalf("retrying: %s %v", status, err)
	}
	job.Attempt = 3
	if err := process.Work(ctx, job); err == nil {
		t.Fatal("invalid video accepted")
	}
	_, status, err = db.MediaLifecycle(ctx, pool, id)
	if err != nil || status != "failed" {
		t.Fatalf("terminal: %s %v", status, err)
	}
	if started, err := db.StartIngestion(ctx, pool, id, generation); err != nil || started {
		t.Fatalf("terminal restarted: %v %v", started, err)
	}
	// A finishing job still owns its uniqueness slot. Repair must wait for it.
	if response := request(http.MethodPost, "/api/admin/regenerate", nil); response.Code != 200 {
		t.Fatal(response.Code)
	}
	_, status, _ = db.MediaLifecycle(ctx, pool, id)
	if status != "failed" {
		t.Fatal("repair requeued while River job still active")
	}
	if _, err := pool.Exec(ctx, `UPDATE river_job SET state='cancelled', finalized_at=now() WHERE kind='process_media' AND args->>'key'=$1`, id); err != nil {
		t.Fatal(err)
	}
	if response := request(http.MethodPost, "/api/admin/regenerate", nil); response.Code != 200 {
		t.Fatalf("repair: %d %s", response.Code, response.Body)
	}
	_, status, err = db.MediaLifecycle(ctx, pool, id)
	if err != nil || status != "queued" {
		t.Fatalf("repair did not requeue: %s %v", status, err)
	}
	if response := request(http.MethodDelete, "/api/media/"+id, nil); response.Code != 204 {
		t.Fatalf("delete: %d %s", response.Code, response.Body)
	}
	if response := request(http.MethodGet, "/api/media/"+id, nil); response.Code != 404 {
		t.Fatalf("deleted detail: %d", response.Code)
	}
	var cleanupJobs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM river_job WHERE kind='cleanup_media' AND args->>'key'=$1`, id).Scan(&cleanupJobs); err != nil || cleanupJobs != 1 {
		t.Fatalf("cleanup not durable: %d %v", cleanupJobs, err)
	}
	// Make file cleanup fail after removing the original.
	if err := os.MkdirAll(store.PreviewPath(id), 0755); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(store.PreviewPath(id), "blocked")
	if err := os.WriteFile(blocker, []byte("block removal"), 0600); err != nil {
		t.Fatal(err)
	}
	cleanupJob := &river.Job[queue.CleanupArgs]{Args: queue.CleanupArgs{Key: id, Generation: generation}}
	if err := cleanup.Work(ctx, cleanupJob); err == nil {
		t.Fatal("expected cleanup failure")
	}
	// Recreate an orphan original to reproduce the original resurrection bug.
	if err := store.PutOriginal(ctx, id, bytes.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	if response := request(http.MethodPost, "/api/admin/regenerate", nil); response.Code != 200 {
		t.Fatalf("repair after cleanup failure: %d", response.Code)
	}
	if exists, err := db.MediaExists(ctx, pool, id); err != nil || exists {
		t.Fatalf("deleted item resurrected: %v %v", exists, err)
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	if err := cleanup.Work(ctx, cleanupJob); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.OriginalPath(id)); !os.IsNotExist(err) {
		t.Fatalf("cleanup left original: %v", err)
	}
	// Re-upload explicitly restores the content with a fresh generation.
	if response := request(http.MethodPost, "/api/media", content); response.Code != 201 {
		t.Fatalf("reupload: %d %s", response.Code, response.Body)
	}
	newGeneration, _, err := db.MediaLifecycle(ctx, pool, id)
	if err != nil || newGeneration == generation {
		t.Fatalf("generation not replaced: %s %v", newGeneration, err)
	}
	// A delayed cleanup job must not remove the replacement original.
	if err := cleanup.Work(ctx, cleanupJob); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.OriginalPath(id)); err != nil {
		t.Fatalf("old cleanup removed replacement: %v", err)
	}
	if err := db.RecordIngestionError(ctx, pool, id, generation, "failed", errors.New("old job")); err != nil {
		t.Fatal(err)
	}
	_, status, err = db.MediaLifecycle(ctx, pool, id)
	if err != nil || status != "queued" {
		t.Fatalf("stale attempt changed replacement: %s %v", status, err)
	}
	// Cleanup and a fresh upload can race in either order. The publication lock
	// must leave the explicitly uploaded generation and its original intact.
	if response := request(http.MethodDelete, "/api/media/"+id, nil); response.Code != 204 {
		t.Fatalf("second delete: %d", response.Code)
	}
	start := make(chan struct{})
	cleanupResult := make(chan error, 1)
	uploadResult := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		<-start
		cleanupResult <- cleanup.Work(ctx, &river.Job[queue.CleanupArgs]{Args: queue.CleanupArgs{Key: id, Generation: newGeneration}})
	}()
	go func() { <-start; uploadResult <- request(http.MethodPost, "/api/media", content) }()
	close(start)
	if err := <-cleanupResult; err != nil {
		t.Fatal(err)
	}
	if response := <-uploadResult; response.Code != 201 {
		t.Fatalf("concurrent reupload: %d %s", response.Code, response.Body)
	}
	if _, err := os.Stat(store.OriginalPath(id)); err != nil {
		t.Fatalf("concurrent cleanup removed replacement: %v", err)
	}
	if current, _, err := db.MediaLifecycle(ctx, pool, id); err != nil || current == newGeneration {
		t.Fatalf("concurrent replacement: %s %v", current, err)
	}
	// Missing DELETEs are idempotent, and never manufacture deletion markers.
	if response := request(http.MethodDelete, "/api/media/00000000000000000000000000000000", nil); response.Code != 204 {
		t.Fatalf("missing delete: %d", response.Code)
	}
	if _, _, err := db.MediaLifecycle(ctx, pool, "00000000000000000000000000000000"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal(err)
	}
}
