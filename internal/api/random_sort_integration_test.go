package api

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sort"
	"testing"
	"time"

	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/search"
	"github.com/gin-gonic/gin"
)

func TestRandomMediaSorting(t *testing.T) {
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
	tag := fmt.Sprintf("random-%d", time.Now().UnixNano())
	ids := make([]string, 9)
	defer func() {
		pool.Exec(ctx, `DELETE FROM media WHERE id=ANY($1)`, ids)
		pool.Exec(ctx, `DELETE FROM tag WHERE name=$1`, tag)
	}()
	for i := range ids {
		ids[i] = fmt.Sprintf("%s-%d", tag, i)
		status := "ready"
		if i == 8 {
			status = "queued"
		}
		if _, err := pool.Exec(ctx, `INSERT INTO media(id,generation,status,format,width,height,size,original_name) VALUES($1,'test',$2,'png',10,10,1,'random.png')`, ids[i], status); err != nil {
			t.Fatal(err)
		}
		if err := db.SetMediaTags(ctx, pool, ids[i], []string{tag}); err != nil {
			t.Fatal(err)
		}
	}
	mediaIDs := func(items []db.Media) []string {
		out := make([]string, len(items))
		for i, m := range items {
			out[i] = m.ID
		}
		return out
	}
	var firstOrder []string
	for _, seed := range []string{"alpha", "bravo", "any-length-seed"} {
		want := slices.Clone(ids[:8])
		sort.Slice(want, func(i, j int) bool {
			return fmt.Sprintf("%x", md5.Sum([]byte(seed+":"+want[i]))) < fmt.Sprintf("%x", md5.Sum([]byte(seed+":"+want[j])))
		})
		items, next, err := search.RandomMedia(ctx, pool, tag, seed, 60, "")
		if err != nil || next != "" || !slices.Equal(mediaIDs(items), want) {
			t.Fatalf("seed=%s items=%v next=%q err=%v", seed, mediaIDs(items), next, err)
		}
		if seed == "alpha" {
			firstOrder = want
		}
		var pages []string
		cursor := ""
		for range 4 {
			items, cursor, err = search.RandomMedia(ctx, pool, tag, seed, 3, cursor)
			if err != nil {
				t.Fatal(err)
			}
			pages = append(pages, mediaIDs(items)...)
			if cursor == "" {
				break
			}
		}
		if !slices.Equal(pages, want) {
			t.Fatalf("pagination repeated or skipped media: %v, want %v", pages, want)
		}
	}
	items, _, err := search.RandomMedia(ctx, pool, tag+" format=webm", "alpha", 60, "")
	if err != nil || len(items) != 0 {
		t.Fatalf("filter ignored: %+v, err=%v", items, err)
	}
	_, cursor, err := search.RandomMedia(ctx, pool, tag, "alpha", 3, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := search.RandomMedia(ctx, pool, tag, "bravo", 3, cursor); !errors.Is(err, search.ErrInvalidCursor) {
		t.Fatalf("cross-seed cursor accepted: %v", err)
	}
	r := gin.New()
	r.GET("/api/media/previews", listPreviewsHandler(pool, nil))
	request := func(query string, wantStatus int) *httptest.ResponseRecorder {
		t.Helper()
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/media/previews?q="+tag+"&sort=random&"+query, nil))
		if res.Code != wantStatus {
			t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
		}
		return res
	}
	res := request("seed=alpha&page_size=3", http.StatusOK)
	var response struct {
		Media      []struct{ ID string }
		NextCursor string `json:"next_cursor"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, m := range response.Media {
		got = append(got, m.ID)
	}
	if !slices.Equal(got, firstOrder[:3]) {
		t.Fatalf("API did not use random sorting: %v", got)
	}
	request("", http.StatusOK)
	request("seed=bravo&cursor="+response.NextCursor, http.StatusBadRequest)
}
