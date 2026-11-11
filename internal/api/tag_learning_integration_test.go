package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/queue"
	"era/booru/internal/workers/tagworker"
	"github.com/gin-gonic/gin"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/riverqueue/river"
)

func TestTagLearningEndpoints(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to an isolated test database")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, &config.Config{PostgresDSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tag := fmt.Sprintf("api-learning-%d", time.Now().UnixNano())
	defer pool.Exec(ctx, `DELETE FROM tag WHERE name=$1`, tag)
	ids := make([]string, 2)
	v := make([]float32, 768)
	v[0] = 1
	for i := range ids {
		id, err := newGeneration()
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = id
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.InsertQueuedMedia(ctx, tx, id, "generation", "test.png", 10, time.Now()); err != nil {
			t.Fatal(err)
		}
		if err := db.CompleteIngestionTx(ctx, tx, db.Media{ID: id, Generation: "generation", Format: "png", Width: 10, Height: 10, Size: 10}); err != nil {
			t.Fatal(err)
		}
		if err := db.ImportEmbeddingTx(ctx, tx, id, "generation", pgvector.NewVector(v)); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		defer func() {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Error(err)
				return
			}
			defer tx.Rollback(ctx)
			if _, err := db.MarkDeletedTx(ctx, tx, id); err != nil {
				t.Error(err)
				return
			}
			if err := tx.Commit(ctx); err != nil {
				t.Error(err)
			}
			pool.Exec(ctx, `DELETE FROM deleted_media WHERE id=$1`, id)
		}()
	}
	if err := db.SetMediaTags(ctx, pool, ids[0], []string{tag}); err != nil {
		t.Fatal(err)
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, &tagworker.RebuildWorker{DB: pool})
	client, err := queue.NewClient(ctx, pool, workers, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	RegisterTagLearningRoutes(r, pool, client)
	request := func(method, path, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("%s %s returned %d: %s", method, path, res.Code, res.Body.String())
		}
		return res
	}
	path := "/api/media/" + ids[1]
	recommendations := func() db.TagRecommendations {
		t.Helper()
		res := request(http.MethodGet, path+"/tag-recommendations", "", http.StatusOK)
		var data db.TagRecommendations
		if err := json.Unmarshal(res.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		return data
	}
	if len(recommendations().LowEvidence) == 0 || len(recommendations().Recommended) != 0 {
		t.Fatal("expected single-example tag in low-evidence suggestions only")
	}
	request(http.MethodPost, path+"/tag-feedback", `{"tag":"tagme","action":"reject"}`, http.StatusBadRequest)
	request(http.MethodPost, path+"/tag-feedback", fmt.Sprintf(`{"tag":%q,"action":"unknown"}`, tag), http.StatusBadRequest)
	request(http.MethodPost, path+"/tag-feedback", fmt.Sprintf(`{"tag":%q,"action":"reject"}`, tag), http.StatusNoContent)
	data := recommendations()
	if len(data.Negative) != 1 || data.Negative[0] != tag {
		t.Fatalf("negative feedback missing: %+v", data)
	}
	request(http.MethodPost, path+"/tag-feedback", fmt.Sprintf(`{"tag":%q,"action":"clear"}`, tag), http.StatusNoContent)
	if len(recommendations().Negative) != 0 {
		t.Fatal("negative feedback was not undone")
	}
	request(http.MethodPost, path+"/tag-feedback", fmt.Sprintf(`{"tag":%q,"action":"accept"}`, tag), http.StatusNoContent)
	item, err := db.GetMedia(ctx, pool, ids[1])
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range item.Tags {
		if entry.Name == tag {
			found = true
		}
	}
	if !found {
		t.Fatal("accepted recommendation was not added to actual tags")
	}
	request(http.MethodGet, "/api/media/not-an-id/tag-recommendations", "", http.StatusBadRequest)
	request(http.MethodGet, "/api/media/00000000000000000000000000000000/tag-recommendations", "", http.StatusNotFound)
	// Duplicate rebuild requests enqueue one durable job while it is pending.
	for range 2 {
		request(http.MethodPost, "/api/settings/tag-statistics/rebuild", "", http.StatusAccepted)
	}
	var jobs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM river_job WHERE kind='rebuild_tag_statistics' AND state='available'`).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatalf("duplicate rebuild jobs: %d, err=%v", jobs, err)
	}
	request(http.MethodGet, "/api/settings/tag-statistics", "", http.StatusOK)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer client.Stop(ctx)
	deadline := time.Now().Add(10 * time.Second)
	for {
		res := request(http.MethodGet, "/api/settings/tag-statistics", "", http.StatusOK)
		var status struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if status.State == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("rebuild did not complete: %s", res.Body.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
}
