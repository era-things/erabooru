package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/wait"

	"era/booru/internal/config"
	common "era/booru/internal/integration/common"
	"era/booru/internal/server"

	"time"
)

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

	common.WaitForPostgres(dsn, 30*time.Second)

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
	mediaWorker, err := common.StartMediaWorker(ctx, cfg)
	if err != nil {
		t.Fatalf("media worker: %v", err)
	}
	defer mediaWorker.Stop()

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()
	client := ts.Client()

	img1Hash, err := common.UploadAndWait(ctx, srv.Minio, client, ts.URL, filepath.Join("testdata", "img1.png"))
	if err != nil {
		t.Fatalf("upload img1: %v", err)
	}
	img2Hash, err := common.UploadAndWait(ctx, srv.Minio, client, ts.URL, filepath.Join("testdata", "img2.png"))
	if err != nil {
		t.Fatalf("upload img2: %v", err)
	}
	img3Hash, err := common.UploadAndWait(ctx, srv.Minio, client, ts.URL, filepath.Join("testdata", "img3.png"))
	if err != nil {
		t.Fatalf("upload img3: %v", err)
	}

	if err := common.AddTags(client, ts.URL, img1Hash, []string{"alpha"}); err != nil {
		t.Fatalf("tags: %v", err)
	}
	if err := common.AddTags(client, ts.URL, img2Hash, []string{"beta"}); err != nil {
		t.Fatalf("tags: %v", err)
	}
	if err := common.AddTags(client, ts.URL, img3Hash, []string{"gamma"}); err != nil {
		t.Fatalf("tags: %v", err)
	}

	if err := common.SetUploadDate(client, ts.URL, img1Hash, "2021-01-02"); err != nil {
		t.Fatalf("set date: %v", err)
	}
	if err := common.SetUploadDate(client, ts.URL, img2Hash, "2022-02-03"); err != nil {
		t.Fatalf("set date: %v", err)
	}

	resp, err := client.Get(ts.URL + "/api/admin/export-tags")
	if err != nil {
		t.Fatalf("export request: %v", err)
	}
	first, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export response %d", resp.StatusCode)
	}
	items1, err := common.ParseExport(first)
	if err != nil {
		t.Fatalf("parse export: %v", err)
	}
	t.Logf("First export has %d items", len(items1))

	srv.Close()
	mediaWorker.Stop()
	ts.Close()

	// purge db via container
	if code, out, err := pgC.Exec(ctx, []string{
		"psql", "-U", "booru", "-d", "booru",
		"-c", "DROP SCHEMA public CASCADE; CREATE SCHEMA public;",
	}); err != nil || code != 0 {
		t.Fatalf("reset via container Exec failed (%d): %s", code, out)
	}

	time.Sleep(2 * time.Second)

	srv, err = server.New(ctx, cfg)
	if err != nil {
		t.Fatalf("restart server: %v", err)
	}
	defer srv.Close()

	mediaWorker, err = common.StartMediaWorker(ctx, cfg)
	if err != nil {
		t.Fatalf("restart media worker: %v", err)
	}
	defer mediaWorker.Stop()

	// Create new HTTP test server
	ts = httptest.NewServer(srv.Router)
	defer ts.Close()
	client = ts.Client()

	// Now regenerate should work without River errors
	resp, err = client.Post(ts.URL+"/api/admin/regenerate", "application/json", nil)
	if err != nil {
		t.Fatalf("regenerate request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("regenerate response %d", resp.StatusCode)
	}

	if err := common.WaitForMedia(client, ts.URL, img1Hash, 10*time.Second); err != nil {
		t.Fatalf("wait img1: %v", err)
	}
	if err := common.WaitForMedia(client, ts.URL, img2Hash, 10*time.Second); err != nil {
		t.Fatalf("wait img2: %v", err)
	}
	if err := common.WaitForMedia(client, ts.URL, img3Hash, 10*time.Second); err != nil {
		t.Fatalf("wait img3: %v", err)
	}

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
	items2, err := common.ParseExport(second)
	if err != nil {
		t.Fatalf("parse second export: %v", err)
	}

	sort.Slice(items1, func(i, j int) bool { return items1[i]["id"].(string) < items1[j]["id"].(string) })
	sort.Slice(items2, func(i, j int) bool { return items2[i]["id"].(string) < items2[j]["id"].(string) })

	if !reflect.DeepEqual(items1, items2) {
		t.Fatalf("export/import mismatch")
	}
}
