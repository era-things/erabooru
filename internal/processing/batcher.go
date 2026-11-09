package processing

import (
	"context"
	"fmt"
	"time"
)

type frameResult struct {
	vector          []float32
	err             error
	batch           uint64
	inference, wait time.Duration
	batchSize       int
}

type frameRequest struct {
	ctx    context.Context
	frame  []byte
	result chan frameResult
}

// FrameBatcher combines prepared images and video frames across durable jobs.
// Only the bounded in-memory preparation window is transient; River owns retries.
type FrameBatcher struct {
	ctx      context.Context
	requests chan frameRequest
	done     chan struct{}
}

func NewFrameBatcher(ctx context.Context, limiter *InferenceLimiter, maximum int, target time.Duration, infer func([][]byte) ([][]float32, error)) *FrameBatcher {
	maximum = max(1, maximum)
	b := &FrameBatcher{ctx: ctx, requests: make(chan frameRequest, maximum), done: make(chan struct{})}
	go func() {
		defer close(b.done)
		b.run(limiter, maximum, target, infer)
	}()
	return b
}

func (b *FrameBatcher) Wait() { <-b.done }

func (b *FrameBatcher) Embed(ctx context.Context, frames [][]byte) ([][]float32, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make([]chan frameResult, len(frames))
	for i, frame := range frames {
		results[i] = make(chan frameResult, 1)
		select {
		case b.requests <- frameRequest{ctx, frame, results[i]}:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-b.ctx.Done():
			return nil, b.ctx.Err()
		}
	}
	seen := make(map[uint64]bool)
	vectors := make([][]float32, len(frames))
	for i, result := range results {
		select {
		case r := <-result:
			if timings := Timings(ctx); timings != nil {
				timings.Frames++
				if !seen[r.batch] {
					seen[r.batch] = true
					timings.Inference += r.inference
					timings.InferenceWait += r.wait
					timings.Batches++
					timings.MaxBatch = max(timings.MaxBatch, r.batchSize)
				}
			}
			if r.err != nil {
				return nil, r.err
			}
			vectors[i] = r.vector
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-b.ctx.Done():
			return nil, b.ctx.Err()
		}
	}
	return vectors, nil
}

func (b *FrameBatcher) run(limiter *InferenceLimiter, maximum int, target time.Duration, infer func([][]byte) ([][]float32, error)) {
	var batchID uint64
	limit := 1 // Learn batch cost without starting with a large uninterruptible call.
	for {
		var first frameRequest
		select {
		case <-b.ctx.Done():
			return
		case first = <-b.requests:
		}
		batch := []frameRequest{first}
		timer := time.NewTimer(5 * time.Millisecond)
	collect:
		for len(batch) < limit {
			select {
			case next := <-b.requests:
				batch = append(batch, next)
			case <-timer.C:
				break collect
			case <-b.ctx.Done():
				timer.Stop()
				return
			}
		}
		timer.Stop()
		started := time.Now()
		release, err := limiter.Acquire(b.ctx, BackgroundInference)
		if err != nil {
			return
		}
		wait := time.Since(started)
		// Requests may have been cancelled while waiting behind searches.
		live := batch[:0]
		for _, request := range batch {
			if request.ctx.Err() == nil {
				live = append(live, request)
			}
		}
		batch = live
		if len(batch) == 0 {
			release()
			continue
		}
		frames := make([][]byte, len(batch))
		for i := range batch {
			frames[i] = batch[i].frame
		}
		started = time.Now()
		vectors, err := infer(frames)
		elapsed := time.Since(started)
		release()
		if err == nil && len(vectors) != len(batch) {
			err = fmt.Errorf("expected %d embeddings, got %d", len(batch), len(vectors))
		}
		batchID++
		for i, request := range batch {
			r := frameResult{err: err, batch: batchID, inference: elapsed, wait: wait, batchSize: len(batch)}
			if err == nil {
				r.vector = vectors[i]
			}
			request.result <- r
		}
		if elapsed > target {
			limit = max(1, len(batch)/2)
		} else if err == nil && len(batch) == limit && elapsed < target/2 {
			limit = min(maximum, limit+1)
		}
	}
}
