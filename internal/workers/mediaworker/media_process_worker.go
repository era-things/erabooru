package mediaworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"era/booru/internal/config"
	"era/booru/internal/db"
	embed "era/booru/internal/embeddings"
	"era/booru/internal/processing"
	"era/booru/internal/queue"
	"era/booru/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// ProcessWorker processes uploaded media objects.
type ProcessWorker struct {
	river.WorkerDefaults[queue.ProcessArgs]
	Store   *storage.Store
	DB      *pgxpool.Pool
	Limiter processing.Limiter
}

func (w *ProcessWorker) Work(ctx context.Context, job *river.Job[queue.ProcessArgs]) error {
	release, err := w.Limiter.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	log.Printf("Processing task started for key %s, content type %s", job.Args.Key, job.Args.ContentType)
	if strings.HasPrefix(job.Args.ContentType, "video/") {
		_, err = w.processVideo(ctx, job.Args.Key)
		return err
	}
	_, err = w.processImage(ctx, job.Args.Key)
	return err
}

func (w *ProcessWorker) saveMediaToDB(ctx context.Context, key, format string, width, height, duration int) error {
	stat, err := os.Stat(w.Store.OriginalPath(key))
	if err != nil {
		return err
	}
	var durationValue *int
	if duration > 0 {
		durationValue = &duration
	}
	return db.SaveMedia(ctx, w.DB, db.Media{
		ID: key, Format: format, Width: width, Height: height,
		Duration: durationValue, Size: stat.Size(), UploadedAt: time.Now().UTC(),
	})
}

// Simplified processImage function
func (w *ProcessWorker) processImage(ctx context.Context, key string) (string, error) {
	rc, err := os.Open(w.Store.OriginalPath(key))
	if err != nil {
		return "", err
	}
	meta, err := processing.GetMetadata(rc)
	_ = rc.Close()
	if err != nil {
		if errors.Is(err, image.ErrFormat) {
			log.Printf("Unsupported image format for %s: %v", key, err)
			return "", river.JobCancel(fmt.Errorf("unsupported image format: %w", err))
		}
		return "", err
	}
	if err := w.generatePreview(ctx, key, ""); err != nil {
		return "", err
	}

	if err := w.saveMediaToDB(ctx, key, meta.Format, meta.Width, meta.Height, 0); err != nil {
		log.Printf("Failed to save media to database: %v", err)
		return "", err
	}
	log.Printf("Saved media %s to database with format %s, width %d, height %d", key, meta.Format, meta.Width, meta.Height)

	enqueued, err := w.enqueueEmbedding(ctx, key)
	if err != nil {
		log.Printf("Failed to enqueue embed job for %s: %v", key, err)
		return "", err
	}
	if enqueued {
		log.Printf("Successfully enqueued embed job for %s", key)
	}

	return key, nil
}

// Simplified processVideo function
func (w *ProcessWorker) processVideo(ctx context.Context, key string) (string, error) {
	src := w.Store.OriginalPath(key)

	probeOut, err := exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", "-show_format", src).Output()
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

	seek := ""
	if duration > 0 {
		seek = strconv.FormatFloat(min(float64(duration)/2, 2), 'f', 3, 64)
	}
	if err := w.generatePreview(ctx, key, seek); err != nil {
		return "", err
	}

	if err := w.saveMediaToDB(ctx, key, format, width, height, duration); err != nil {
		return "", err
	}

	enqueued, err := w.enqueueEmbedding(ctx, key)
	if err != nil {
		log.Printf("Failed to enqueue embed job for %s: %v", key, err)
		return "", err
	}
	if enqueued {
		log.Printf("Successfully enqueued embed job for %s", key)
	}

	return key, nil
}

func (w *ProcessWorker) enqueueEmbedding(ctx context.Context, key string) (bool, error) {
	if !embed.Available() {
		return false, nil
	}
	return true, queue.WorkerEnqueue(ctx, queue.EmbedArgs{Key: key})
}

func (w *ProcessWorker) generatePreview(ctx context.Context, key, seek string) error {
	args := []string{"-loglevel", "error"}
	if seek != "" {
		args = append(args, "-ss", seek)
	}
	args = append(args,
		"-i", w.Store.OriginalPath(key), "-frames:v", "1",
		"-vf", "scale=512:-2:force_original_aspect_ratio=decrease",
		"-q:v", "4", "-f", "image2", "pipe:1",
	)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if err = w.Store.PutPreview(ctx, key, stdout); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	return cmd.Wait()
}
