package mediaworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"era/booru/ent"
	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/minio"
	"era/booru/internal/processing"
	"era/booru/internal/queue"

	mc "github.com/minio/minio-go/v7"
	"github.com/riverqueue/river"
)

// ProcessWorker processes uploaded media objects.
type ProcessWorker struct {
	river.WorkerDefaults[queue.ProcessArgs]
	Minio *minio.Client
	DB    *ent.Client
	Cfg   *config.Config
}

func (w *ProcessWorker) Work(ctx context.Context, job *river.Job[queue.ProcessArgs]) error {
	log.Printf("Processing task started for bucket %s, key %s, content type %s", job.Args.Bucket, job.Args.Key, job.Args.ContentType)
	bucket := job.Args.Bucket
	if bucket == "" {
		bucket = w.Minio.Bucket
	}
	if strings.HasPrefix(job.Args.ContentType, "video/") {
		_, err := w.processVideo(ctx, bucket, job.Args.Key)
		return err
	}
	_, err := w.processImage(ctx, bucket, job.Args.Key)
	return err
}

func (w *ProcessWorker) saveMediaToDB(ctx context.Context, key, format string, width, height, duration int) error {
	tx, err := w.DB.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Find or create the tagme tag first
	tagme, err := db.FindOrCreateTag(ctx, tx.Client(), "tagme")
	if err != nil {
		return err
	}

	mediaCreate := tx.Media.Create().
		SetID(key).
		SetFormat(format).
		SetWidth(int16(width)).
		SetHeight(int16(height))

	// Only set duration for videos
	if duration > 0 {
		mediaCreate = mediaCreate.SetDuration(int16(duration))
	}

	// Add tags during creation instead of after
	mediaCreate = mediaCreate.AddTagIDs(tagme.ID)

	mediaObj, err := mediaCreate.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			// Media already exists, just return success
			return nil
		}
		return err
	}

	// Create upload date record
	dt, err := db.FindOrCreateDate(ctx, tx.Client(), "upload")
	if err != nil {
		return err
	}
	if _, err := tx.MediaDate.Create().
		SetMediaID(mediaObj.ID).
		SetDateID(dt.ID).
		SetValue(time.Now().UTC()).
		Save(ctx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if err := queue.WorkerEnqueue(ctx, queue.IndexArgs{ID: key}); err != nil {
		log.Printf("Failed to enqueue index job for ID %s: %v", key, err)
		return err
	}

	return nil
}

// Simplified processImage function
func (w *ProcessWorker) processImage(ctx context.Context, bucket, key string) (string, error) {
	rc, err := w.Minio.GetObject(ctx, bucket, key, mc.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}

	meta, err := processing.GetMetadata(data)
	if err != nil {
		if errors.Is(err, image.ErrFormat) {
			log.Printf("Unsupported image format for %s: %v", key, err)
			return "", river.JobCancel(fmt.Errorf("unsupported image format: %w", err))
		}
		return "", err
	}

	// Use common database save function
	if err := w.saveMediaToDB(ctx, key, meta.Format, meta.Width, meta.Height, 0); err != nil {
		log.Printf("Failed to save media to database: %v", err)
		return "", err
	}
	log.Printf("Saved media %s to database with format %s, width %d, height %d", key, meta.Format, meta.Width, meta.Height)

	if err := queue.WorkerEnqueue(ctx, queue.EmbedArgs{Bucket: bucket, Key: key}); err != nil {
		log.Printf("Failed to enqueue embed job for %s: %v", key, err)
		return "", err
	}
	log.Printf("Successfully enqueued embed job for %s", key)

	return key, nil
}

// Simplified processVideo function
func (w *ProcessWorker) processVideo(ctx context.Context, bucket, key string) (string, error) {
	scheme := "http://"
	if w.Cfg.MinioSSL {
		scheme = "https://"
	}
	src := fmt.Sprintf("%s%s/%s/%s", scheme, w.Cfg.MinioInternalEndpoint, strings.TrimPrefix(bucket, "/"), strings.TrimPrefix(key, "/"))

	probeOut, err := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", "-show_format", src).Output()
	if err != nil {
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
		return "", err
	}
	width, height, duration := 0, 0, 0
	for _, s := range probe.Streams {
		if s.CodecType == "video" {
			if s.Width > width {
				width = s.Width
			}
			if s.Height > height {
				height = s.Height
			}
			if s.Duration != "" {
				if f, err := strconv.ParseFloat(s.Duration, 64); err == nil {
					if int(f+0.5) > duration {
						duration = int(f + 0.5)
					}
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
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	previewKey := key
	if _, err = w.Minio.PutPreviewJpeg(ctx, previewKey, stdout); err != nil {
		return "", err
	}
	if err := cmd.Wait(); err != nil {
		return "", err
	}

	obj, err := w.Minio.GetObject(ctx, bucket, key, mc.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer obj.Close()

	// Use common database save function
	if err := w.saveMediaToDB(ctx, key, format, width, height, duration); err != nil {
		return "", err
	}

	if err := queue.WorkerEnqueue(ctx, queue.EmbedArgs{Bucket: bucket, Key: key}); err != nil {
		log.Printf("Failed to enqueue embed job for %s: %v", key, err)
		return "", err
	}
	log.Printf("Successfully enqueued embed job for %s", key)

	return key, nil
}
