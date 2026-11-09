package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ErabooruClient provides helpers for interacting with the test server.
// All fatal errors are reported via the testing.TB instance.
type ErabooruClient struct {
	t       testing.TB
	client  *http.Client
	baseURL string
}

// NewClient constructs a new helper client.
func NewClient(t testing.TB, httpClient *http.Client, baseURL string) *ErabooruClient {
	t.Helper()
	return &ErabooruClient{t: t, client: httpClient, baseURL: strings.TrimRight(baseURL, "/")}
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

// UploadAndWait uploads the given file and waits until it is processed.
func (c *ErabooruClient) UploadAndWait(ctx context.Context, path string) string {
	c.t.Helper()
	f, err := os.Open(path)
	if err != nil {
		c.t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": filepath.Base(path)})},
		"Content-Type":        {"image/png"},
	})
	if err != nil {
		c.t.Fatal(err)
	}
	if _, err := io.Copy(part, f); err != nil {
		c.t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		c.t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/media", &body)
	if err != nil {
		c.t.Fatalf("create upload request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := c.client.Do(req)
	if err != nil {
		c.t.Fatalf("upload %s: %v", path, err)
	}
	if resp.StatusCode != http.StatusCreated {
		resp.Body.Close()
		c.t.Fatalf("upload response %d", resp.StatusCode)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		resp.Body.Close()
		c.t.Fatalf("decode upload response: %v", err)
	}
	resp.Body.Close()
	c.WaitForMedia(result.ID, 10*time.Second)
	return result.ID
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
