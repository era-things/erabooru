package processing

import (
	"context"
	"log"
	"time"
)

// MediaTimings follows an item through ingestion and its durable embedding job.
// Only the worker owning the context updates it; batch results return timings
// to that worker rather than modifying it from the inference goroutine.
type MediaTimings struct {
	Started       time.Time     `json:"started"`
	Preview       time.Duration `json:"preview"`
	DecodeWait    time.Duration `json:"decode_wait"`
	Prepare       time.Duration `json:"prepare"`
	Decode        time.Duration `json:"decode"`
	Inference     time.Duration `json:"inference"`
	InferenceWait time.Duration `json:"inference_wait"`
	Frames        int           `json:"frames"`
	Batches       int           `json:"batches"`
	MaxBatch      int           `json:"max_batch"`
}

type timingsKey struct{}

func WithTimings(ctx context.Context, timings *MediaTimings) context.Context {
	return context.WithValue(ctx, timingsKey{}, timings)
}

func Timings(ctx context.Context) *MediaTimings {
	timings, _ := ctx.Value(timingsKey{}).(*MediaTimings)
	return timings
}

func (t *MediaTimings) Log(key, stage string, err error) {
	ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	log.Printf("benchmark key=%s stage=%s total_ms=%.2f preview_ms=%.2f prepare_ms=%.2f decode_ms=%.2f decode_wait_ms=%.2f inference_ms=%.2f inference_wait_ms=%.2f frames=%d batches=%d max_batch=%d error=%q", key, stage, ms(time.Since(t.Started)), ms(t.Preview), ms(t.Prepare), ms(t.Decode), ms(t.DecodeWait), ms(t.Inference), ms(t.InferenceWait), t.Frames, t.Batches, t.MaxBatch, errorText(err))
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
