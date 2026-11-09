package api

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"era/booru/internal/storage"
	"github.com/gin-gonic/gin"
)

func mediaUploadRequest(t *testing.T, name, contentType string, data []byte, extra bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": name})},
		"Content-Type":        {contentType},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if extra {
		if err := w.WriteField("extra", "invalid"); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/media", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestMediaFilePreservesOriginalName(t *testing.T) {
	for _, test := range []struct{ input, expected string }{
		{"2026-11-09T13-04-05.123Z.png", "2026-11-09T13-04-05.123Z.png"},
		{" снимок — 日本語 🖼️.png", " снимок — 日本語 🖼️.png"},
		{"../../saved.png", "saved.png"},
		{`C:\fakepath\saved.png`, "saved.png"},
	} {
		t.Run(test.input, func(t *testing.T) {
			data := []byte("unchanged original bytes")
			req := mediaUploadRequest(t, test.input, "image/png", data, false)
			reader, _ := req.MultipartReader()
			part, _ := reader.NextPart()
			contentType, name, err := readMediaFile(part)
			if err != nil || name != test.expected || contentType != "image/png" {
				t.Fatalf("got %q %q %v", name, contentType, err)
			}
			actual, _ := io.ReadAll(part)
			if !bytes.Equal(actual, data) {
				t.Fatal("original bytes changed")
			}
		})
	}
}

func TestUploadRejectsMissingNameAndObsoleteRawBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/media", uploadMediaHandler(nil, nil, nil))
	requests := []*http.Request{
		mediaUploadRequest(t, "", "image/png", []byte("media"), false),
		mediaUploadRequest(t, "..", "image/png", []byte("media"), false),
		mediaUploadRequest(t, "name.txt", "text/plain", []byte("media"), false),
		httptest.NewRequest(http.MethodPost, "/api/media", bytes.NewReader([]byte("old raw upload"))),
	}
	for _, req := range requests {
		response := httptest.NewRecorder()
		r.ServeHTTP(response, req)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("accepted invalid request: %d %s", response.Code, response.Body)
		}
	}
}

func TestUploadRejectsExtraPartsWithoutLeavingOriginal(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.POST("/api/media", uploadMediaHandler(nil, store, nil))
	response := httptest.NewRecorder()
	r.ServeHTTP(response, mediaUploadRequest(t, "saved.png", "image/png", []byte("media"), true))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("extra part accepted: %d", response.Code)
	}
	count := 0
	if err := store.WalkOriginals(func(string) error { count++; return nil }); err != nil || count != 0 {
		t.Fatalf("invalid upload left originals: %d %v", count, err)
	}
}
