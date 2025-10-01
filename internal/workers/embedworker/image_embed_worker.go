package embedworker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

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

var visionEmbedding = embed.VisionEmbedding

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

		data, err := io.ReadAll(obj)
		if err != nil {
			log.Printf("Failed to read image object: %v", err)
			return err
		}

		vec, err = embed.VisionEmbedding(data)
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

	logEmbedQueueDepth(ctx, fmt.Sprintf("Successfully generated and saved embedding for key %s (job %d)", job.Args.Key, job.ID))
	return nil
}

func (w *ImageEmbedWorker) videoEmbedding(ctx context.Context, bucket, key string, media *ent.Media) (vec []float32, retErr error) {
	if w.Cfg == nil {
		return nil, fmt.Errorf("missing worker configuration")
	}

	startTime := time.Now()

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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	vectors := make([][]float32, 0, samples)
	var mu sync.Mutex
	var fatalErr error
	var fatalMu sync.Mutex

	concurrency := runtime.NumCPU()
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > samples {
		concurrency = samples
	}
	if concurrency == 0 {
		concurrency = 1
	}

	frameCh, waitStream, err := streamVideoFrames(ctx, src, duration, samples)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := waitStream(); err != nil {
			if retErr == nil {
				retErr = err
			}
			vec = nil
		}
	}()

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	processed := 0

	for frame := range frameCh {
		fatalMu.Lock()
		if fatalErr != nil {
			fatalMu.Unlock()
			continue
		}
		fatalMu.Unlock()

		data := frame
		idx := processed
		processed++

		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, frame []byte) {
			defer wg.Done()
			defer func() { <-sem }()

			vec, err := visionEmbedding(frame)
			if err != nil {
				fatalMu.Lock()
				if fatalErr == nil {
					fatalErr = fmt.Errorf("failed to embed frame %d: %w", idx, err)
					cancel()
				}
				fatalMu.Unlock()
				return
			}

			mu.Lock()
			vectors = append(vectors, vec)
			mu.Unlock()
		}(idx, data)
	}

	wg.Wait()

	if fatalErr != nil {
		return nil, fatalErr
	}

	if len(vectors) == 0 {
		return nil, fmt.Errorf("no frames extracted for video %s", key)
	}

	avg, err := averageVectors(vectors)
	if err != nil {
		return nil, err
	}

	elapsed := time.Since(startTime).Milliseconds()
	log.Printf("Video embedding for %s: processed %d frames in %d ms", key, len(vectors), elapsed)

	vec = avg
	return vec, nil
}

func streamVideoFrames(ctx context.Context, src string, durationSeconds, samples int) (<-chan []byte, func() error, error) {
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

	args := []string{
		"-i", src,
		"-vf", fmt.Sprintf("fps=%d/%d", samples, denom),
		"-vframes", strconv.Itoa(samples),
		"-f", "image2pipe",
		"-vcodec", "png",
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
		err := readPNGFrames(reader, frameCh)
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

var pngSignature = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

func readPNGFrames(r *bufio.Reader, out chan<- []byte) error {
	for {
		frame, err := readPNGFrame(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		out <- frame
	}
}

func readPNGFrame(r *bufio.Reader) ([]byte, error) {
	header := make([]byte, len(pngSignature))
	if _, err := io.ReadFull(r, header); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, err
	}
	if !bytes.Equal(header, pngSignature) {
		return nil, fmt.Errorf("unexpected PNG signature")
	}

	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	if _, err := buf.Write(header); err != nil {
		return nil, err
	}

	for {
		chunkHeader := make([]byte, 8)
		if _, err := io.ReadFull(r, chunkHeader); err != nil {
			return nil, err
		}
		if _, err := buf.Write(chunkHeader); err != nil {
			return nil, err
		}

		length := binary.BigEndian.Uint32(chunkHeader[:4])
		data := make([]byte, length+4)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}
		if _, err := buf.Write(data); err != nil {
			return nil, err
		}

		if string(chunkHeader[4:8]) == "IEND" {
			break
		}
	}

	return buf.Bytes(), nil
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
