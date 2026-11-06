package embedworker

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
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
	Store     *storage.Store
	DB        *pgxpool.Pool
	Inference *processing.InferenceLimiter
}

func (w *ImageEmbedWorker) Work(ctx context.Context, job *river.Job[queue.EmbedArgs]) (retErr error) {
	started, err := db.StartEmbedding(ctx, w.DB, job.Args.Key, job.Args.Generation, job.Args.Space)
	if err != nil {
		return err
	}
	if !started {
		return river.JobCancel(fmt.Errorf("stale embedding generation for %s", job.Args.Key))
	}
	defer func() {
		if retErr != nil {
			db.FailEmbedding(context.WithoutCancel(ctx), w.DB, job.Args.Key, job.Args.Generation, job.Args.Space, retErr)
		}
	}()
	log.Printf("Generating embedding for key %s", job.Args.Key)

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
	if config.SupportedVideoFormats[format] {
		vec, err = w.videoEmbedding(ctx, job.Args.Key, media)
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
			log.Printf("Failed to read original: %v", err)
			return classifyObjectError(err, job.Args.Key)
		}

		release, acquireErr := w.Inference.Acquire(ctx, processing.BackgroundInference)
		if acquireErr != nil {
			return acquireErr
		}
		vec, err = embed.VisionEmbedding(data)
		release()
		if err != nil {
			log.Printf("Failed to generate embedding: %v", err)
			return classifyEmbeddingError(err, job.Args.Key)
		}
	}

	pgv := pgvector.NewVector(vec)
	if err := db.CompleteEmbedding(ctx, w.DB, job.Args.Key, job.Args.Generation, job.Args.Space, pgv); err != nil {
		log.Printf("Failed to save embedding to database: %v", err)
		return err
	}

	log.Printf("Successfully generated and saved embedding for key %s (job %d)", job.Args.Key, job.ID)
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

func (w *ImageEmbedWorker) videoEmbedding(ctx context.Context, key string, media db.Media) ([]float32, error) {
	startTime := time.Now()

	duration := 0
	if media.Duration != nil && *media.Duration > 0 {
		duration = int(*media.Duration)
	}

	samples := videoSampleCount(duration)
	if samples <= 0 {
		return nil, fmt.Errorf("invalid sample count computed for video %s", key)
	}

	src := w.Store.OriginalPath(key)
	if _, err := os.Stat(src); err != nil {
		return nil, err
	}

	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		inputSize := embed.InputSpatialSize()
		frames := make([][]byte, 0, samples)
		frameCh, waitStream, err := streamVideoFrames(ctx, src, duration, samples, inputSize)
		if err != nil {
			return nil, err
		}

		for frame := range frameCh {
			frames = append(frames, frame)
		}
		if err := waitStream(); err != nil {
			return nil, err
		}
		if len(frames) == 0 {
			return nil, fmt.Errorf("no frames extracted for video %s", key)
		}
		vectors, err := embedVideoFrames(ctx, w.Inference, frames, embed.VisionEmbeddingsRGB24)
		if err != nil {
			lastErr = err
			if newSize, ok := embed.RetargetInputSpatialSize(err, inputSize); ok {
				log.Printf("Video embedding input resized from %d to %d for %s", inputSize, newSize, key)
				continue
			}
			return nil, err
		}

		avg, err := averageVectors(vectors)
		if err != nil {
			return nil, err
		}

		elapsed := time.Since(startTime).Milliseconds()
		log.Printf("Video embedding for %s: processed %d frames in %d ms", key, len(vectors), elapsed)
		return avg, nil
	}
	return nil, lastErr
}

// Embed one frame per admission so a video cannot hold the inference budget
// across every sampled frame while a search is waiting.
func embedVideoFrames(ctx context.Context, limiter *processing.InferenceLimiter, frames [][]byte, infer func([][]byte) ([][]float32, error)) ([][]float32, error) {
	vectors := make([][]float32, 0, len(frames))
	for i := range frames {
		vector, err := func() ([]float32, error) {
			release, err := limiter.Acquire(ctx, processing.BackgroundInference)
			if err != nil {
				return nil, err
			}
			defer release()
			batch, err := infer(frames[i : i+1])
			if err != nil {
				return nil, err
			}
			if len(batch) != 1 {
				return nil, fmt.Errorf("expected one frame embedding, got %d", len(batch))
			}
			return batch[0], nil
		}()
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, vector)
	}
	return vectors, nil
}

func streamVideoFrames(ctx context.Context, src string, durationSeconds, samples, inputSize int) (<-chan []byte, func() error, error) {
	if samples <= 0 {
		return nil, nil, fmt.Errorf("invalid frame sample count")
	}

	denom := durationSeconds
	if denom <= 0 {
		denom = samples
		if denom <= 0 {
			denom = 1
		}
	}

	S := inputSize
	if S <= 0 {
		return nil, nil, fmt.Errorf("invalid vision input size %d", S)
	}

	frameSize := 3 * S * S
	if frameSize <= 0 {
		return nil, nil, fmt.Errorf("invalid frame size computed for %d", S)
	}

	vf := fmt.Sprintf(
		"fps=%d/%d,scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(%d-iw)/2:(%d-ih)/2:color=black",
		samples,
		denom,
		S,
		S,
		S,
		S,
		S,
		S,
	)

	args := []string{
		"-i", src,
		"-vf", vf,
		"-vframes", strconv.Itoa(samples),
		"-pix_fmt", "rgb24",
		"-f", "rawvideo",
		"-loglevel", "error",
		"-",
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create ffmpeg stdout pipe: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	frameCh := make(chan []byte)
	errCh := make(chan error, 2)
	doneCh := make(chan struct{})

	go func() {
		reader := bufio.NewReader(stdout)
		err := readRawVideoFrames(reader, frameSize, samples, frameCh)
		if err != nil {
			errCh <- err
		} else {
			errCh <- nil
		}
		close(frameCh)
	}()

	go func() {
		err := cmd.Wait()
		if err != nil {
			if stderr.Len() > 0 {
				err = fmt.Errorf("ffmpeg error: %w: %s", err, strings.TrimSpace(stderr.String()))
			} else {
				err = fmt.Errorf("ffmpeg error: %w", err)
			}
		}
		errCh <- err
		close(doneCh)
	}()

	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = cmd.Process.Signal(syscall.SIGTERM)
				select {
				case <-doneCh:
				case <-time.After(2 * time.Second):
					_ = cmd.Process.Kill()
				}
			}
		case <-doneCh:
		}
	}()

	wait := func() error {
		var firstErr error
		for i := 0; i < 2; i++ {
			err := <-errCh
			if err != nil {
				if errors.Is(err, io.EOF) {
					continue
				}
				if firstErr == nil {
					firstErr = err
				}
			}
		}
		return firstErr
	}

	return frameCh, wait, nil
}

func readRawVideoFrames(r *bufio.Reader, frameSize, samples int, out chan<- []byte) error {
	if frameSize <= 0 {
		return fmt.Errorf("invalid frame size %d", frameSize)
	}
	if samples <= 0 {
		return fmt.Errorf("invalid sample count %d", samples)
	}

	for i := 0; i < samples; i++ {
		frame := make([]byte, frameSize)
		if _, err := io.ReadFull(r, frame); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}
		out <- frame
	}

	return nil
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
