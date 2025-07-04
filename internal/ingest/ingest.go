package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
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
	scheme := "http://"
	if cfg.MinioSSL {
		scheme = "https://"
	}
	src := fmt.Sprintf("%s%s/%s/%s", scheme,
		cfg.MinioInternalEndpoint,
		strings.TrimPrefix(m.Bucket, "/"),
		strings.TrimPrefix(object, "/"))

	probeOut, err := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", "-show_format", src).Output()
	if err != nil {
		log.Printf("ffprobe %s: %v", object, err)
		return "", err
	}
	var probe struct {
		Streams []struct {
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Duration  string `json:"duration"`
			CodecType string `json:"codec_type"`
		} `json:"streams"`
		Format struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(probeOut, &probe); err != nil {
		log.Printf("probe decode %s: %v", object, err)
		return "", err
	}
	width, height := 0, 0
	duration := 0
	for _, s := range probe.Streams {
		if s.CodecType == "video" {
			if s.Width > width {
				width = s.Width
			}
			if s.Height > height {
				height = s.Height
			}
			if s.Duration != "" {
				if f, err := strconv.ParseFloat(s.Duration, 64); err == nil && int(f+0.5) > duration {
					duration = int(f + 0.5)
				}
			}
		}
	}
	if duration == 0 && probe.Format.Duration != "" {
		if f, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
			duration = int(f + 0.5)
		}
	}
	format := probe.Format.FormatName
	for f := range config.SupportedVideoFormats {
		if strings.Contains(format, f) {
			format = f
			break
		}
	}

	cmd := exec.Command("ffmpeg",
		"-ss", "00:00:02", "-i", src, "-y", "-loglevel", "error",
		"-vframes", "1", "-vf", "scale=320:-2",
		"-q:v", "3", "-f", "image2", "pipe:1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("pipe %s: %v", object, err)
		return "", err
	}
	if err := cmd.Start(); err != nil {
		log.Printf("ffmpeg start %s: %v", object, err)
		return "", err
	}
	previewKey := object
	if _, err := m.PutPreviewJpeg(ctx, previewKey, stdout); err != nil {
		log.Printf("upload preview %s: %v", object, err)
		return "", err
	}
	if err := cmd.Wait(); err != nil {
		log.Printf("ffmpeg wait %s: %v", object, err)
		return "", err
	}

	obj, err := m.GetObject(ctx, m.Bucket, object, mc.GetObjectOptions{})
	if err != nil {
		log.Printf("get object %s: %v", object, err)
		return "", err
	}
	defer obj.Close()
	h := sha256.New()
	if _, err := io.Copy(h, obj); err != nil {
		log.Printf("hash %s: %v", object, err)
		return "", err
	}
	_ = hex.EncodeToString(h.Sum(nil))

	media, err := db.Media.Create().
		SetID(object).
		SetFormat(format).
		SetWidth(int16(width)).
		SetHeight(int16(height)).
		SetDuration(int16(duration)).
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
