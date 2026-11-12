package embedworker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
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
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/riverqueue/river"
)

// ImageEmbedWorker generates vision embeddings for images.
type ImageEmbedWorker struct {
	river.WorkerDefaults[queue.EmbedArgs]
	Store   *storage.Store
	DB      *pgxpool.Pool
	Batcher *processing.FrameBatcher
	Decode  processing.Limiter
}

func (w *ImageEmbedWorker) Work(ctx context.Context, job *river.Job[queue.EmbedArgs]) (retErr error) {
	timings := job.Args.Timings
	if timings.Started.IsZero() {
		timings.Started = time.Now()
	}
	ctx = processing.WithTimings(ctx, &timings)
	defer func() { timings.Log(job.Args.Key, "embedding", retErr) }()
	started, err := db.StartEmbedding(ctx, w.DB, job.Args.Key, job.Args.Generation, job.Args.Space)
	if err != nil {
		return err
	}
	if !started {
		return river.JobCancel(fmt.Errorf("stale embedding generation for %s", job.Args.Key))
	}
	defer func() {
		if retErr != nil {
			failureCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if err := db.RecordEmbeddingError(failureCtx, w.DB, job.Args.Key, job.Args.Generation, job.Args.Space, queue.FailureStatus(job.JobRow, retErr), retErr); err != nil {
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

	media, err := db.GetMedia(ctx, w.DB, job.Args.Key)
	if errors.Is(err, pgx.ErrNoRows) {
		log.Printf("Media metadata missing for %s: %v", job.Args.Key, err)
		return river.JobCancel(fmt.Errorf("media %s not found: %w", job.Args.Key, err))
	}
	if err != nil {
		log.Printf("Failed to load media metadata: %v", err)
		return err
	}

	format := strings.ToLower(media.Format)

	var vec []float32
	var inputSize int
	if config.SupportedVideoFormats[format] {
		vec, inputSize, err = w.videoEmbedding(ctx, job.Args.Key, media)
		if err != nil {
			log.Printf("Failed to generate video embedding: %v", err)
			return classifyEmbeddingError(err, job.Args.Key)
		}
	} else {
		// Ingestion already bounded and oriented this image for its preview.
		// Embedding the derived image avoids decoding a potentially enormous
		// original for a second time without changing the model input size.
		data, err := os.ReadFile(w.Store.PreviewPath(job.Args.Key))
		if err != nil {
			log.Printf("Failed to read preview: %v", err)
			return classifyObjectError(err, job.Args.Key)
		}

		preparedAt := time.Now()
		inputSize = embed.InputSpatialSize()
		raw, prepareErr := embed.PrepareImage(data, inputSize)
		timings.Prepare += time.Since(preparedAt)
		if prepareErr != nil {
			return classifyEmbeddingError(prepareErr, job.Args.Key)
		}
		vectors, inferErr := w.Batcher.Embed(ctx, [][]byte{raw})
		if inferErr != nil {
			return classifyEmbeddingError(inferErr, job.Args.Key)
		}
		vec = vectors[0]
	}

	pgv := pgvector.NewVector(vec)
	if err := db.CompleteEmbedding(ctx, w.DB, job.Args.Key, job.Args.Generation, job.Args.Space, pgv, embed.ActiveProfile().ForImageSize(inputSize).ID); err != nil {
		log.Printf("Failed to save embedding to database: %v", err)
		return err
	}

	return nil
}

func classifyObjectError(err error, key string) error {
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return river.JobCancel(fmt.Errorf("object %s missing: %w", key, err))
	}
	return err
}

func classifyEmbeddingError(err error, key string) error {
	if err == nil {
		return nil
	}
	if isFatalEmbeddingError(err) {
		return river.JobCancel(fmt.Errorf("embedding cannot be generated for %s: %w", key, err))
	}
	return err
}

func isFatalEmbeddingError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	fatalSubstrings := []string{
		"Corrupt JPEG data",
		"no frames extracted",
		"unexpected raw buffer length",
		"invalid sample count",
		"invalid vision input size",
	}
	for _, sub := range fatalSubstrings {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	if strings.Contains(msg, "vips thumbnail") && strings.Contains(msg, "VipsJpeg") {
		return true
	}
	return false
}

func (w *ImageEmbedWorker) videoEmbedding(ctx context.Context, key string, media db.Media) ([]float32, int, error) {

	duration := 0
	if media.Duration != nil && *media.Duration > 0 {
		duration = int(*media.Duration)
	}

	samples := videoSampleCount(duration)
	if samples <= 0 {
		return nil, 0, fmt.Errorf("invalid sample count computed for video %s", key)
	}

	src := w.Store.OriginalPath(key)
	if _, err := os.Stat(src); err != nil {
		return nil, 0, err
	}

	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		inputSize := embed.InputSpatialSize()
		waitAt := time.Now()
		release, err := w.Decode.Acquire(ctx)
		if err != nil {
			return nil, 0, err
		}
		decodeWait := time.Since(waitAt)
		decodeAt := time.Now()
		frames, err := extractVideoFrames(ctx, src, duration, samples, inputSize)
		release()
		if timings := processing.Timings(ctx); timings != nil {
			timings.DecodeWait += decodeWait
			timings.Decode += time.Since(decodeAt)
		}
		if err != nil {
			return nil, 0, err
		}
		if len(frames) == 0 {
			return nil, 0, fmt.Errorf("no frames extracted for video %s", key)
		}
		vectors, err := w.Batcher.Embed(ctx, frames)
		if err != nil {
			lastErr = err
			if newSize, ok := embed.RetargetInputSpatialSize(err, inputSize); ok {
				log.Printf("Video embedding input resized from %d to %d for %s", inputSize, newSize, key)
				continue
			}
			return nil, 0, err
		}

		avg, err := averageVectors(vectors)
		if err != nil {
			return nil, 0, err
		}

		return avg, inputSize, nil
	}
	return nil, 0, lastErr
}

// Long clips use sparse input-side seeks. Short/unknown clips retain a single
// sequential decode to avoid paying process startup and GOP decoding per sample.
func videoSamplingMode(duration int) string {
	if duration >= 60 {
		return "seek"
	}
	return "sequential"
}

func samplingDenominator(duration, samples int) int {
	if duration > 0 {
		return duration
	}
	return samples
}

func videoFrameArgs(src string, duration, samples, size int, seek *float64) []string {
	args := []string{"-loglevel", "error", "-nostdin", "-threads", processing.FFmpegThreads()}
	if seek != nil {
		args = append(args, "-ss", strconv.FormatFloat(*seek, 'f', 6, 64))
	}
	// Automatic selection prefers resolution and can pick a larger secondary
	// track that has no frames at the requested timestamps.
	args = append(args, "-i", src, "-map", "0:v:0", "-an", "-sn", "-dn", "-filter_threads", "1")
	vf := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(%d-iw)/2:(%d-ih)/2:color=black", size, size, size, size, size, size)
	count := samples
	if seek != nil {
		count = 1
	} else {
		vf = fmt.Sprintf("fps=%d/%d,", samples, samplingDenominator(duration, samples)) + vf
	}
	return append(args, "-vf", vf, "-frames:v", strconv.Itoa(count), "-pix_fmt", "rgb24", "-threads", processing.FFmpegThreads(), "-f", "rawvideo", "pipe:1")
}

func extractVideoFrames(ctx context.Context, src string, duration, samples, size int) ([][]byte, error) {
	if samples < 1 || size < 1 {
		return nil, fmt.Errorf("invalid video sampling dimensions")
	}
	frames := make([][]byte, 0, samples)
	calls := 1
	if videoSamplingMode(duration) == "seek" {
		calls = samples
	}
	for i := 0; i < calls; i++ {
		var seek *float64
		if calls > 1 {
			timestamp := (float64(i) + 0.5) * float64(duration) / float64(samples)
			seek = &timestamp
		}
		cmd := exec.CommandContext(ctx, "ffmpeg", videoFrameArgs(src, duration, samples, size, seek)...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("ffmpeg frame extraction: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		frameBytes := 3 * size * size
		if len(output)%frameBytes != 0 {
			return nil, fmt.Errorf("unexpected raw buffer length %d", len(output))
		}
		for offset := 0; offset < len(output); offset += frameBytes {
			frames = append(frames, output[offset:offset+frameBytes])
		}
	}
	return frames, nil
}

func videoSampleCount(durationSeconds int) int {
	switch {
	case durationSeconds < 5:
		return 5
	case durationSeconds < 60:
		return 8
	case durationSeconds < 300:
		return 12
	default:
		return 16
	}
}

func averageVectors(vectors [][]float32) ([]float32, error) {
	if len(vectors) == 0 {
		return nil, fmt.Errorf("no vectors to average")
	}
	dim := len(vectors[0])
	if dim == 0 {
		return nil, fmt.Errorf("vector dimension is zero")
	}
	avg := make([]float32, dim)
	for i, vec := range vectors {
		if len(vec) != dim {
			return nil, fmt.Errorf("vector %d has mismatched dimension %d", i, len(vec))
		}
		for j, v := range vec {
			avg[j] += v
		}
	}
	scale := 1 / float32(len(vectors))
	for i := range avg {
		avg[i] *= scale
	}

	var sumSquares float64
	for _, v := range avg {
		sumSquares += float64(v) * float64(v)
	}
	if sumSquares > 0 {
		norm := float32(1 / math.Sqrt(sumSquares))
		for i := range avg {
			avg[i] *= norm
		}
	}

	return avg, nil
}
