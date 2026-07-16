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
	"github.com/testcontainers/testcontainers-go/wait"

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
		Image:        "pgvector/pgvector:pg16",
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

	common.WaitForPostgres(t, dsn, 30*time.Second)
	cfg := common.SetupEnv(t, dsn)

	srv, err := server.New(ctx, cfg)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	defer srv.Close()

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()
	client := ts.Client()
	ec := common.NewClient(t, client, ts.URL)

	img1Hash := ec.UploadAndWait(ctx, filepath.Join("testdata", "img1.png"))
	img2Hash := ec.UploadAndWait(ctx, filepath.Join("testdata", "img2.png"))
	img3Hash := ec.UploadAndWait(ctx, filepath.Join("testdata", "img3.png"))

	ec.AddTags(img1Hash, []string{"alpha"})
	ec.AddTags(img2Hash, []string{"beta"})
	ec.AddTags(img3Hash, []string{"gamma"})

	ec.SetUploadDate(img1Hash, "2021-01-02")
	ec.SetUploadDate(img2Hash, "2022-02-03")

	resp, err := client.Get(ts.URL + "/api/admin/export-tags")
	if err != nil {
		t.Fatalf("export request: %v", err)
	}
	first, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export response %d", resp.StatusCode)
	}
	items1 := common.ParseExport(t, first)
	t.Logf("First export has %d items", len(items1))

	srv.Close()
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

	// Create new HTTP test server
	ts = httptest.NewServer(srv.Router)
	defer ts.Close()
	client = ts.Client()
	ec = common.NewClient(t, client, ts.URL)

	// Now regenerate should work without River errors
	resp, err = client.Post(ts.URL+"/api/admin/regenerate", "application/json", nil)
	if err != nil {
		t.Fatalf("regenerate request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("regenerate response %d", resp.StatusCode)
	}

	ec.WaitForMedia(img1Hash, 10*time.Second)
	ec.WaitForMedia(img2Hash, 10*time.Second)
	ec.WaitForMedia(img3Hash, 10*time.Second)

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
	items2 := common.ParseExport(t, second)

	sort.Slice(items1, func(i, j int) bool { return items1[i]["id"].(string) < items1[j]["id"].(string) })
	sort.Slice(items2, func(i, j int) bool { return items2[i]["id"].(string) < items2[j]["id"].(string) })

	if !reflect.DeepEqual(items1, items2) {
		t.Fatalf("export/import mismatch")
	}
}
