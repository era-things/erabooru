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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// ProcessWorker processes uploaded media objects.
type ProcessWorker struct {
	river.WorkerDefaults[queue.ProcessArgs]
	Store  *storage.Store
	DB     *pgxpool.Pool
	Decode processing.Limiter
}

func (w *ProcessWorker) Work(ctx context.Context, job *river.Job[queue.ProcessArgs]) (retErr error) {
	timings := &processing.MediaTimings{Started: time.Now()}
	ctx = processing.WithTimings(ctx, timings)
	handedOff := false
	defer func() {
		if !handedOff {
			timings.Log(job.Args.Key, "ingestion", retErr)
		}
	}()
	started, err := db.StartIngestion(ctx, w.DB, job.Args.Key, job.Args.Generation)
	if err != nil {
		return err
	}
	if !started {
		return river.JobCancel(fmt.Errorf("stale ingestion generation for %s", job.Args.Key))
	}
	defer func() {
		if retErr != nil {
			failureCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if err := db.RecordIngestionError(failureCtx, w.DB, job.Args.Key, job.Args.Generation, queue.FailureStatus(job.JobRow, retErr), retErr); err != nil {
				log.Printf("record job failure: %v", err)
			}
		}
	}()

	// Keep application status aligned with River even when processing panics.
	defer func() {
		if recovered := recover(); recovered != nil {
			retErr = fmt.Errorf("media processing panic: %v", recovered)
		}
	}()

	var item db.Media
	if strings.HasPrefix(job.Args.ContentType, "video/") {
		item, err = w.processVideo(ctx, job.Args.Key, job.Args.Generation)
	} else {
		item, err = w.processImage(ctx, job.Args.Key, job.Args.Generation)
	}
	if err != nil {
		return err
	}
	item.Generation = job.Args.Generation
	handedOff, err = w.completeIngestion(ctx, item)
	return err
}

func (w *ProcessWorker) processImage(ctx context.Context, key, generation string) (db.Media, error) {
	rc, err := os.Open(w.Store.OriginalPath(key))
	if err != nil {
		return db.Media{}, err
	}
	meta, err := processing.GetMetadata(rc)
	_ = rc.Close()
	if err != nil {
		if errors.Is(err, image.ErrFormat) {
			log.Printf("Unsupported image format for %s: %v", key, err)
			return db.Media{}, river.JobCancel(fmt.Errorf("unsupported image format: %w", err))
		}
		return db.Media{}, err
	}
	if err := w.generatePreview(ctx, key, generation, ""); err != nil {
		return db.Media{}, err
	}
	stat, err := os.Stat(w.Store.OriginalPath(key))
	if err != nil {
		return db.Media{}, err
	}
	return db.Media{ID: key, Format: meta.Format, Width: meta.Width, Height: meta.Height, Size: stat.Size()}, nil
}

// Simplified processVideo function
func (w *ProcessWorker) processVideo(ctx context.Context, key, generation string) (db.Media, error) {
	src := w.Store.OriginalPath(key)

	probeOut, err := exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", "-show_format", src).Output()
	if err != nil {
		return db.Media{}, err
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
		return db.Media{}, err
	}
	width, height, duration := 0, 0, 0
	for _, s := range probe.Streams {
		if s.CodecType == "video" {
			// Metadata, preview and embedding all use the first video track.
			width, height = s.Width, s.Height
			if s.Duration != "" {
				if f, err := strconv.ParseFloat(s.Duration, 64); err == nil {
					if int(f+0.5) > duration {
						duration = int(f + 0.5)
					}
				}
			}
			break
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
	if err := w.generatePreview(ctx, key, generation, seek); err != nil {
		return db.Media{}, err
	}
	stat, err := os.Stat(src)
	if err != nil {
		return db.Media{}, err
	}
	var durationValue *int
	if duration > 0 {
		durationValue = &duration
	}
	return db.Media{ID: key, Format: format, Width: width, Height: height, Duration: durationValue, Size: stat.Size()}, nil
}

func (w *ProcessWorker) completeIngestion(ctx context.Context, item db.Media) (bool, error) {
	tx, err := w.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if err := db.LockMediaTx(ctx, tx, item.ID); err != nil {
		return false, err
	}
	if err := db.CompleteIngestionTx(ctx, tx, item); err != nil {
		return false, err
	}
	if embed.Available() {
		space := db.DefaultEmbeddingSpace
		queued, err := db.QueueEmbeddingTx(ctx, tx, item.ID, item.Generation, space)
		if err != nil {
			return false, err
		}
		if !queued {
			return false, tx.Commit(ctx)
		}
		client := river.ClientFromContext[pgx.Tx](ctx)
		if client == nil {
			return false, river.ErrNotFound
		}
		if err := queue.EnqueueTx(ctx, client, tx, queue.EmbedArgs{Key: item.ID, Generation: item.Generation, Space: space, Timings: *processing.Timings(ctx)}); err != nil {
			return false, err
		}
		err = tx.Commit(ctx)
		return err == nil, err
	}
	return false, tx.Commit(ctx)
}

func (w *ProcessWorker) generatePreview(ctx context.Context, key, generation, seek string) error {
	tx, err := w.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if err := db.LockMediaTx(ctx, tx, key); err != nil {
		return err
	}
	current, status, err := db.MediaLifecycle(ctx, tx, key)
	if err != nil || current != generation || status != "processing" {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return river.JobCancel(fmt.Errorf("stale preview generation for %s", key))
	}
	started := time.Now()
	release, err := w.Decode.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	waited := time.Since(started)
	started = time.Now()
	defer func() {
		if timings := processing.Timings(ctx); timings != nil {
			timings.DecodeWait += waited
			timings.Preview += time.Since(started)
		}
	}()
	cmd := exec.CommandContext(ctx, "ffmpeg", previewArgs(w.Store.OriginalPath(key), seek)...)
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

func previewArgs(src, seek string) []string {
	args := []string{"-loglevel", "error", "-nostdin", "-threads", processing.FFmpegThreads()}
	if seek != "" {
		args = append(args, "-ss", seek)
	}
	return append(args,
		"-i", src, "-map", "0:v:0", "-frames:v", "1", "-filter_threads", "1",
		"-vf", "scale=512:512:force_original_aspect_ratio=decrease",
		"-q:v", "4", "-threads", processing.FFmpegThreads(), "-f", "image2", "pipe:1",
	)
}
