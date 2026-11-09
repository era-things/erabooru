package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	embed "era/booru/internal/embeddings"
	"github.com/gin-gonic/gin"
)

func TestImportedEmbeddingValidation(t *testing.T) {
	profile := embed.Profile{ID: "test-profile", Dimensions: 3}
	for _, test := range []struct {
		name, body      string
		valid, mismatch bool
	}{
		{"valid", `{"profile":"test-profile","vector":[3,4,0]}`, true, false},
		{"wrong model", `{"profile":"other","vector":[3,4,0]}`, false, true},
		{"dimension", `{"profile":"test-profile","vector":[1,2]}`, false, false},
		{"zero", `{"profile":"test-profile","vector":[0,0,0]}`, false, false},
		{"null", `{"profile":"test-profile","vector":[1,null,0]}`, false, false},
		{"overflow", `{"profile":"test-profile","vector":[1e100,0,0]}`, false, false},
		{"string", `{"profile":"test-profile","vector":["1",0,0]}`, false, false},
		{"unknown", `{"profile":"test-profile","vector":[1,0,0],"extra":true}`, false, false},
		{"trailing", `{"profile":"test-profile","vector":[1,0,0]} {}`, false, false},
		{"oversize", strings.Repeat(" ", 65537), false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			vector, err := readImportedEmbedding(strings.NewReader(test.body), profile)
			if test.valid {
				if err != nil {
					t.Fatal(err)
				}
				if math.Abs(float64(vector[0])-.6) > 1e-6 || math.Abs(float64(vector[1])-.8) > 1e-6 {
					t.Fatalf("not normalized: %v", vector)
				}
			} else if err == nil {
				t.Fatal("invalid embedding accepted")
			}
			if test.mismatch && !errors.Is(err, errEmbeddingProfile) {
				t.Fatal(err)
			}
		})
	}
}

func importRequest(t *testing.T, metadata string, fileFirst bool, extra bool, files ...[]byte) (*http.Request, []byte) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	file := []byte("test media content")
	if len(files) > 0 {
		file = files[0]
	}
	addFile := func() {
		header := textproto.MIMEHeader{"Content-Disposition": {`form-data; name="file"; filename="sample.mp4"`}, "Content-Type": {"video/mp4"}}
		if len(files) > 0 {
			header.Set("Content-Type", http.DetectContentType(file))
		}
		p, err := w.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = p.Write(file); err != nil {
			t.Fatal(err)
		}
	}
	if fileFirst {
		addFile()
	}
	if err := w.WriteField("embedding", metadata); err != nil {
		t.Fatal(err)
	}
	if !fileFirst {
		addFile()
	}
	if extra {
		if err := w.WriteField("extra", "no"); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/media/import", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, file
}

func TestImportRejectsInvalidMetadataBeforeStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		metadata  string
		fileFirst bool
		status    int
	}{
		{`{"profile":"wrong","vector":[1,0,0]}`, false, 409},
		{`{"profile":"test","vector":[0,0,0]}`, false, 422},
		{`{"profile":"test","vector":[1,0,0]}`, true, 422},
	} {
		req, _ := importRequest(t, test.metadata, test.fileFirst, false)
		r := gin.New()
		r.POST("/api/media/import", importMediaHandler(nil, nil, nil, embed.Profile{ID: "test", Dimensions: 3}))
		response := httptest.NewRecorder()
		r.ServeHTTP(response, req)
		if response.Code != test.status {
			t.Fatalf("got %d: %s", response.Code, response.Body)
		}
	}
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	embeddingProfileHandler(embed.Profile{})(ctx)
	if response.Code != 503 {
		t.Fatal("unavailable model advertised")
	}
}

func TestImportMetadataStreamsVideoPart(t *testing.T) {
	metadata, _ := json.Marshal(map[string]any{"profile": "test", "vector": []float32{1, 0, 0}})
	req, file := importRequest(t, string(metadata), false, false)
	reader, err := req.MultipartReader()
	if err != nil {
		t.Fatal(err)
	}
	_, part, contentType, originalName, err := readImportMetadata(reader, embed.Profile{ID: "test", Dimensions: 3})
	if err != nil || contentType != "video/mp4" || originalName != "sample.mp4" {
		t.Fatalf("metadata: %s %v", contentType, err)
	}
	var actual bytes.Buffer
	if _, err = actual.ReadFrom(part); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual.Bytes(), file) {
		t.Fatal("media stream changed")
	}
}
