package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"era/booru/internal/config"
	"era/booru/internal/db"
	"github.com/gin-gonic/gin"
	pgvector "github.com/pgvector/pgvector-go"
)

func TestMetadataExportImportRoundTrip(t *testing.T) {
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
	id := fmt.Sprintf("metadata-%d", time.Now().UnixNano())
	queuedID := id + "-queued"
	positive, negative := id+"-positive", id+"-negative"
	defer pool.Exec(ctx, `DELETE FROM tag WHERE name=ANY($1)`, []string{positive, negative})
	defer pool.Exec(ctx, `DELETE FROM media WHERE id=ANY($1)`, []string{id, queuedID})
	date := time.Date(2021, 2, 3, 4, 5, 6, 123456000, time.UTC)
	for _, key := range []string{id, queuedID} {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := db.InsertQueuedMedia(ctx, tx, key, "generation", "original.png", 10, date); err != nil {
			t.Fatal(err)
		}
		if key == id {
			if err := db.CompleteIngestionTx(ctx, tx, db.Media{ID: key, Generation: "generation", Format: "png", Width: 10, Height: 10, Size: 10}); err != nil {
				t.Fatal(err)
			}
			v := make([]float32, 768)
			v[0] = 1
			if err := db.ImportEmbeddingTx(ctx, tx, key, "generation", pgvector.NewVector(v), "metadata-test"); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SetMediaTags(ctx, pool, id, []string{positive, negative}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetTagFeedback(ctx, pool, id, negative, "reject"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetMediaTags(ctx, pool, queuedID, []string{positive}); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/export", exportTagsHandler(pool))
	r.POST("/import", importTagsHandler(pool))
	export := func() (int, map[string]metadataRecord) {
		t.Helper()
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/export", nil))
		if res.Code != http.StatusOK {
			t.Fatalf("export %d: %s", res.Code, res.Body.String())
		}
		gz, err := gzip.NewReader(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		defer gz.Close()
		dec := json.NewDecoder(gz)
		var meta struct {
			Version int `json:"version"`
		}
		if err := dec.Decode(&meta); err != nil {
			t.Fatal(err)
		}
		items := map[string]metadataRecord{}
		for {
			var raw map[string]json.RawMessage
			if err := dec.Decode(&raw); err == io.EOF {
				break
			} else if err != nil {
				t.Fatal(err)
			}
			if len(raw) != 6 {
				t.Fatalf("unexpected exported fields: %v", raw)
			}
			for _, field := range []string{"id", "original_name", "uploaded_at", "tags", "negative_tags", "dates"} {
				if _, ok := raw[field]; !ok {
					t.Fatalf("missing source field %s", field)
				}
			}
			data, _ := json.Marshal(raw)
			var item metadataRecord
			if err := json.Unmarshal(data, &item); err != nil {
				t.Fatal(err)
			}
			items[item.ID] = item
		}
		return meta.Version, items
	}
	importItems := func(version int, want int, items ...metadataRecord) {
		t.Helper()
		var body bytes.Buffer
		gz := gzip.NewWriter(&body)
		enc := json.NewEncoder(gz)
		if err := enc.Encode(map[string]int{"version": version}); err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			if err := enc.Encode(item); err != nil {
				t.Fatal(err)
			}
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/import", &body))
		if res.Code != want {
			t.Fatalf("import %d: %s", res.Code, res.Body.String())
		}
	}
	version, before := export()
	if version != 3 || len(before[id].NegativeTags) != 1 || before[id].NegativeTags[0] != negative || len(before[queuedID].Tags) != 1 {
		t.Fatalf("missing source metadata: version=%d items=%+v", version, before)
	}
	if before[id].Dates[0].Value != date.Format("2006-01-02") || before[id].UploadedAt == nil || !before[id].UploadedAt.Equal(date) {
		t.Fatalf("date precision lost: %+v", before[id])
	}
	// Remove the negative tag definition too: restoration must recreate rejected-only tags.
	if _, err := pool.Exec(ctx, `DELETE FROM tag WHERE name=$1`, negative); err != nil {
		t.Fatal(err)
	}
	changedName := "changed.png"
	if err := db.RestoreMediaMetadata(ctx, pool, db.MediaMetadata{ID: id, OriginalName: &changedName, Tags: []string{}, NegativeTags: []string{}, Dates: []db.DateValue{}}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		importItems(3, http.StatusOK, before[id], before[queuedID])
		_, after := export()
		if !reflect.DeepEqual(before[id], after[id]) || !reflect.DeepEqual(before[queuedID], after[queuedID]) {
			t.Fatalf("round trip changed metadata: before=%+v after=%+v", before, after)
		}
		var count int
		if err := pool.QueryRow(ctx, `SELECT negative_count FROM tag_statistics s JOIN tag t ON t.id=s.tag_id WHERE t.name=$1 AND profile='metadata-test'`, negative).Scan(&count); err != nil || count != 1 {
			t.Fatalf("derived statistics: count=%d err=%v", count, err)
		}
	}
	legacy := before[id]
	legacy.Dates = append([]metadataDate(nil), legacy.Dates...)
	legacy.NegativeTags = nil
	legacy.OriginalName = nil
	legacy.UploadedAt = nil
	legacy.Dates[0].Value = "2021-02-03"
	importItems(2, http.StatusOK, legacy)
	_, afterLegacy := export()
	if !reflect.DeepEqual(before[id].NegativeTags, afterLegacy[id].NegativeTags) || *afterLegacy[id].OriginalName != "original.png" {
		t.Fatal("legacy import lost metadata")
	}
	invalid := before[id]
	invalid.NegativeTags = []string{positive}
	importItems(3, http.StatusBadRequest, invalid)
	importItems(99, http.StatusBadRequest, before[id])
	_, afterInvalid := export()
	if !reflect.DeepEqual(afterLegacy, afterInvalid) {
		t.Fatal("invalid import modified metadata")
	}
	clear := before[id]
	clear.NegativeTags = []string{}
	importItems(3, http.StatusOK, clear)
	_, afterClear := export()
	if len(afterClear[id].NegativeTags) != 0 {
		t.Fatal("empty rejected list did not clear existing feedback")
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT negative_count FROM tag_statistics s JOIN tag t ON t.id=s.tag_id WHERE t.name=$1 AND profile='metadata-test'`, negative).Scan(&count); err != nil || count != 0 {
		t.Fatalf("clearing feedback did not update statistics: count=%d err=%v", count, err)
	}
	importItems(3, http.StatusOK, metadataRecord{ID: id + "-missing", NegativeTags: []string{}})
}
