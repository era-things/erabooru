package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	mc "github.com/minio/minio-go/v7"

	"era/booru/internal/minio"
	"era/booru/internal/processing"
)

// ErabooruClient provides helpers for interacting with the test server and MiniO.
// All fatal errors are reported via the testing.TB instance.
type ErabooruClient struct {
	t       testing.TB
	Minio   *minio.Client
	client  *http.Client
	baseURL string
}

// NewClient constructs a new helper client.
func NewClient(t testing.TB, m *minio.Client, httpClient *http.Client, baseURL string) *ErabooruClient {
	t.Helper()
	return &ErabooruClient{t: t, Minio: m, client: httpClient, baseURL: strings.TrimRight(baseURL, "/")}
}

// WaitForMedia polls until the media item becomes available.
func (c *ErabooruClient) WaitForMedia(id string, timeout time.Duration) {
	c.t.Helper()
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("%s/api/media/%s", c.baseURL, id)
	for time.Now().Before(deadline) {
		resp, err := c.client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(600 * time.Millisecond)
	}
	c.t.Fatalf("media %s not ingested", id)
}

// UploadAndWait uploads the given file to MinIO and waits until it is processed.
func (c *ErabooruClient) UploadAndWait(ctx context.Context, path string) string {
	c.t.Helper()
	f, err := os.Open(path)
	if err != nil {
		c.t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	hash, err := processing.HashFile128Hex(f)
	if err != nil {
		c.t.Fatalf("hash %s: %v", path, err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		c.t.Fatalf("seek %s: %v", path, err)
	}
	if _, err := c.Minio.PutObject(ctx, c.Minio.Bucket, hash, f, -1, mc.PutObjectOptions{ContentType: "image/png"}); err != nil {
		c.t.Fatalf("put %s: %v", hash, err)
	}
	c.WaitForMedia(hash, 10*time.Second)
	return hash
}

// AddTags adds tags to the given media item via the API.
func (c *ErabooruClient) AddTags(id string, tags []string) {
	c.t.Helper()
	b, _ := json.Marshal(struct {
		Tags []string `json:"tags"`
	}{Tags: tags})
	resp, err := c.client.Post(fmt.Sprintf("%s/api/media/%s/tags", c.baseURL, id), "application/json", bytes.NewReader(b))
	if err != nil {
		c.t.Fatalf("post tags %s: %v", id, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.t.Fatalf("tag response %d", resp.StatusCode)
	}
}

// SetUploadDate sets the upload date for the given media item via the API.
func (c *ErabooruClient) SetUploadDate(id, val string) {
	c.t.Helper()
	b, _ := json.Marshal(struct {
		Dates []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"dates"`
	}{Dates: []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{{Name: "upload", Value: val}}})

	resp, err := c.client.Post(fmt.Sprintf("%s/api/media/%s/dates", c.baseURL, id), "application/json", bytes.NewReader(b))
	if err != nil {
		c.t.Fatalf("post date %s: %v", id, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.t.Fatalf("date response %d", resp.StatusCode)
	}
}
