package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"era/booru/ent"
	"era/booru/internal/config"
	dbpkg "era/booru/internal/db"
	"era/booru/internal/minio"
	"era/booru/internal/processing"

	mc "github.com/minio/minio-go/v7"
)

// AnalyzeImage extracts metadata from an image object and stores it in Postgres.
func AnalyzeImage(ctx context.Context, m *minio.Client, db *ent.Client, object string) (string, error) {
	rc, err := m.GetObject(ctx, m.Bucket, object, mc.GetObjectOptions{})
	if err != nil {
		log.Printf("get object %s: %v", object, err)
		return "", err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		log.Printf("read object %s: %v", object, err)
		return "", err
	}

	metadata, err := processing.GetMetadata(data)
	if err != nil {
		log.Printf("get metadata for %s: %v", object, err)
		return "", err
	}

	media, err := db.Media.Create().
		SetID(object).
		SetFormat(metadata.Format).
		SetWidth(int16(metadata.Width)).
		SetHeight(int16(metadata.Height)).
		Save(ctx)
	if err != nil {
		log.Printf("create media: %v", err)
		return "", err
	}

	dt, err := dbpkg.FindOrCreateDate(ctx, db, "upload")
	if err != nil {
		log.Printf("date lookup: %v", err)
		return "", err
	}
	if _, err := db.MediaDate.Create().
		SetMediaID(media.ID).
		SetDateID(dt.ID).
		SetValue(time.Now().UTC()).
		Save(ctx); err != nil {
		log.Printf("create media date: %v", err)
		return "", err
	}

	log.Printf("saved media %s", object)
	return media.ID, nil
}

// AnalyzeVideo asks the video worker to create a preview and extract metadata
// for the video object, then saves the metadata in Postgres.
func AnalyzeVideo(ctx context.Context, cfg *config.Config, m *minio.Client, db *ent.Client, object string) (string, error) {
	reqBody := struct {
		Bucket string `json:"bucket"`
		Key    string `json:"key"`
	}{Bucket: m.Bucket, Key: object}

	b, _ := json.Marshal(reqBody)
	resp, err := http.Post(cfg.VideoWorkerURL+"/process", "application/json", bytes.NewReader(b))
	if err != nil {
		log.Printf("video worker request: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("video worker status: %s", resp.Status)
		return "", fmt.Errorf("video worker returned status %s", resp.Status)
	}

	var out struct {
		PreviewKey string `json:"preview_key"`
		Format     string `json:"format"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		Duration   int    `json:"duration"`
		Hash       string `json:"hash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Printf("decode video worker response: %v", err)
		return "", err
	}

	media, err := db.Media.Create().
		SetID(reqBody.Key).
		SetFormat(out.Format).
		SetWidth(int16(out.Width)).
		SetHeight(int16(out.Height)).
		SetDuration(int16(out.Duration)).
		Save(ctx)
	if err != nil {
		log.Printf("create video media: %v", err)
		return "", err
	}

	dt, err := dbpkg.FindOrCreateDate(ctx, db, "upload")
	if err != nil {
		log.Printf("date lookup: %v", err)
		return "", err
	}
	if _, err := db.MediaDate.Create().
		SetMediaID(media.ID).
		SetDateID(dt.ID).
		SetValue(time.Now().UTC()).
		Save(ctx); err != nil {
		log.Printf("create media date: %v", err)
		return "", err
	}

	log.Printf("saved video %s", object)
	return media.ID, nil
}

// Process decides whether an object is an image or video and runs the
// appropriate analysis functions. The contentType may be empty; in that case
// the decision is made based on the file extension.
func Process(ctx context.Context, cfg *config.Config, m *minio.Client, db *ent.Client, object, contentType string) (string, error) {
	if strings.HasPrefix(contentType, "video/") {
		return AnalyzeVideo(ctx, cfg, m, db, object)
	} else if strings.HasPrefix(contentType, "image/") {
		return AnalyzeImage(ctx, m, db, object)
	} else {
		err := fmt.Errorf("unsupported content type %s for object %s", contentType, object)
		log.Printf("%v", err)
		return "", err
	}
}
