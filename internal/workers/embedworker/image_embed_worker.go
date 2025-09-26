package embedworker

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"strings"

	"era/booru/ent"
	"era/booru/internal/config"
	"era/booru/internal/db"
	embed "era/booru/internal/embeddings"
	"era/booru/internal/minio"
	"era/booru/internal/queue"

	mc "github.com/minio/minio-go/v7"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/riverqueue/river"
)

// ImageEmbedWorker generates vision embeddings for images.
type ImageEmbedWorker struct {
	river.WorkerDefaults[queue.EmbedArgs]
	Minio *minio.Client
	DB    *ent.Client
	Cfg   *config.Config
}

func (w *ImageEmbedWorker) Work(ctx context.Context, job *river.Job[queue.EmbedArgs]) error {
	log.Printf("Generating embedding for bucket %s, key %s", job.Args.Bucket, job.Args.Key)
	bucket := job.Args.Bucket
	if bucket == "" {
		bucket = w.Minio.Bucket
	}

	media, err := w.DB.Media.Get(ctx, job.Args.Key)
	if err != nil {
		log.Printf("Failed to load media metadata: %v", err)
		return err
	}

	format := strings.ToLower(media.Format)

	var vec []float32
	if config.SupportedVideoFormats[format] {
		vec, err = w.videoEmbedding(ctx, bucket, job.Args.Key, media)
		if err != nil {
			log.Printf("Failed to generate video embedding: %v", err)
			return err
		}
	} else {
		obj, err := w.Minio.GetObject(ctx, bucket, job.Args.Key, mc.GetObjectOptions{})
		if err != nil {
			log.Printf("Failed to get object from MinIO: %v", err)
			return err
		}
		defer obj.Close()

		img, _, err := image.Decode(obj)
		if err != nil {
			log.Printf("Failed to decode image: %v", err)
			return err
		}

		vec, err = embed.VisionEmbedding(img)
		if err != nil {
			log.Printf("Failed to generate embedding: %v", err)
			return err
		}
	}

	pgv := pgvector.NewVector(vec)
	if err := db.SetMediaVectors(ctx, w.DB, job.Args.Key, []db.VectorValue{{Name: "vision", Value: pgv}}); err != nil {
		log.Printf("Failed to save embedding to database: %v", err)
		return err
	}

	if err := queue.WorkerEnqueue(ctx, queue.IndexArgs{ID: job.Args.Key}); err != nil {
		log.Printf("Failed to enqueue reindex for %s: %v", job.Args.Key, err)
	}

	log.Printf("Successfully generated and saved embedding for key %s", job.Args.Key)
	return nil
}

func (w *ImageEmbedWorker) videoEmbedding(ctx context.Context, bucket, key string, media *ent.Media) ([]float32, error) {
	if w.Cfg == nil {
		return nil, fmt.Errorf("missing worker configuration")
	}

	duration := 0
	if media.Duration != nil && *media.Duration > 0 {
		duration = int(*media.Duration)
	}

	samples := videoSampleCount(duration)
	if samples <= 0 {
		return nil, fmt.Errorf("invalid sample count computed for video %s", key)
	}

	src, cleanup, err := w.cachedVideoPath(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	vectors := make([][]float32, 0, samples)
	for i := 0; i < samples; i++ {
		ts := sampleTimestamp(duration, samples, i)
		img, err := extractFrame(ctx, src, ts)
		if err != nil {
			return nil, fmt.Errorf("failed to extract frame %d: %w", i, err)
		}

		vec, err := embed.VisionEmbedding(img)
		if err != nil {
			return nil, fmt.Errorf("failed to embed frame %d: %w", i, err)
		}
		vectors = append(vectors, vec)
	}

	avg, err := averageVectors(vectors)
	if err != nil {
		return nil, err
	}
	return avg, nil
}

func extractFrame(ctx context.Context, src string, timestamp float64) (image.Image, error) {
	ts := formatTimestamp(timestamp)
	args := []string{"-ss", ts, "-i", src, "-frames:v", "1", "-f", "image2", "-vcodec", "png", "-loglevel", "error", "-"}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("ffmpeg error: %v: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("ffmpeg error: %w", err)
	}

	if stdout.Len() == 0 {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("ffmpeg produced no data: %s", strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("ffmpeg produced no data for timestamp %s", ts)
	}

	img, _, err := image.Decode(&stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to decode frame image: %w", err)
	}
	return img, nil
}

func (w *ImageEmbedWorker) cachedVideoPath(ctx context.Context, bucket, key string) (string, func(), error) {
	obj, err := w.Minio.GetObject(ctx, bucket, key, mc.GetObjectOptions{})
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch video object: %w", err)
	}
	defer func() {
		if obj != nil {
			obj.Close()
		}
	}()

	tmp, err := os.CreateTemp("", "video-embed-*.bin")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	name := tmp.Name()

	if _, err := io.Copy(tmp, obj); err != nil {
		tmp.Close()
		os.Remove(name)
		return "", nil, fmt.Errorf("failed to cache video locally: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(name)
		return "", nil, fmt.Errorf("failed to flush video cache: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return "", nil, fmt.Errorf("failed to close cached video: %w", err)
	}

	if err := obj.Close(); err != nil {
		os.Remove(name)
		return "", nil, fmt.Errorf("failed to close video object: %w", err)
	}
	obj = nil

	cleanup := func() {
		os.Remove(name)
	}

	return name, cleanup, nil
}

func formatTimestamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	totalMillis := math.Floor(seconds * 1000)
	if totalMillis < 0 {
		totalMillis = 0
	}
	totalSeconds := int(totalMillis) / 1000
	ms := int(totalMillis) % 1000
	h := totalSeconds / 3600
	m := (totalSeconds % 3600) / 60
	s := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

func sampleTimestamp(durationSeconds int, sampleCount, index int) float64 {
	if durationSeconds <= 0 {
		return 0
	}
	if sampleCount <= 0 {
		return 0
	}

	span := float64(durationSeconds)
	if sampleCount == 1 {
		return span / 2
	}

	step := span / float64(sampleCount)
	ts := (float64(index) + 0.5) * step
	if ts >= span {
		ts = math.Nextafter(span, 0)
	}
	if ts < 0 {
		ts = 0
	}
	return ts
}

func videoSampleCount(durationSeconds int) int {
	switch {
	case durationSeconds <= 0:
		return 5
	case durationSeconds < 5:
		return 5
	case durationSeconds < 60:
		return durationSeconds
	default:
		return 60
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
