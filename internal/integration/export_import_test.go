package integration_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"era/booru/internal/config"
	"era/booru/internal/server"
	mc "github.com/minio/minio-go/v7"
	"time"
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

func TestExportImportCycle(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("integration test; set RUN_INTEGRATION_TESTS=1 to run")
	}

	ctx := context.Background()

	pgC, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("booru"),
		tcpostgres.WithUsername("booru"),
		tcpostgres.WithPassword("booru"))
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	defer pgC.Terminate(ctx)

	if _, err := pgC.ConnectionString(ctx, "sslmode=disable"); err != nil {
		t.Fatalf("postgres conn: %v", err)
	}
	host, err := pgC.Host(ctx)
	if err != nil {
		t.Fatalf("host: %v", err)
	}
	portNat, err := pgC.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("port: %v", err)
	}
	port := portNat.Port()

	mC, err := tcminio.Run(ctx, "minio/minio:RELEASE.2024-01-16T16-07-38Z",
		tcminio.WithUsername("minioadmin"), tcminio.WithPassword("minio123"))
	if err != nil {
		t.Fatalf("start minio: %v", err)
	}
	defer mC.Terminate(ctx)

	minioAddr, err := mC.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("minio addr: %v", err)
	}

	os.Setenv("POSTGRES_HOST", host)
	os.Setenv("POSTGRES_PORT", port)
	os.Setenv("POSTGRES_USER", "booru")
	os.Setenv("POSTGRES_PASSWORD", "booru")
	os.Setenv("POSTGRES_DB", "booru")
	os.Setenv("MINIO_ROOT_USER", "minioadmin")
	os.Setenv("MINIO_ROOT_PASSWORD", "minio123")
	os.Setenv("MINIO_BUCKET", "boorubucket")
	os.Setenv("MINIO_PREVIEW_BUCKET", "previews")
	os.Setenv("MINIO_INTERNAL_ENDPOINT", minioAddr)
	os.Setenv("MINIO_PUBLIC_HOST", "")
	os.Setenv("MINIO_PUBLIC_PREFIX", "")
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

	upload := func(obj, path string) {
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		defer f.Close()
		if _, err := srv.Minio.PutObject(ctx, srv.Minio.Bucket, obj, f, -1, mc.PutObjectOptions{ContentType: "image/png"}); err != nil {
			t.Fatalf("put %s: %v", obj, err)
		}
		waitFor(obj)
	}

	upload("img1.png", filepath.Join("testdata", "img1.png"))
	upload("img2.png", filepath.Join("testdata", "img2.png"))
	upload("img3.png", filepath.Join("testdata", "img3.png"))

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

	addTags("img1.png", []string{"alpha"})
	addTags("img2.png", []string{"beta"})
	addTags("img3.png", []string{"gamma"})

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

	setDate("img1.png", "2021-01-02")
	setDate("img2.png", "2022-02-03")

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

	resp, err = client.Post(ts.URL+"/api/admin/regenerate", "application/json", nil)
	if err != nil {
		t.Fatalf("regenerate request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("regenerate response %d", resp.StatusCode)
	}

	resp, err = client.Post(ts.URL+"/api/admin/import-tags", "application/gzip", bytes.NewReader(first))
	if err != nil {
		t.Fatalf("import request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("import response %d", resp.StatusCode)
	}

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
