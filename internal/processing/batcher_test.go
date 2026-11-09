package processing

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestBatcherCombinesFramesAndPreservesOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	timings := &MediaTimings{}
	ctx = WithTimings(ctx, timings)
	var sizes []int
	b := NewFrameBatcher(ctx, NewInferenceLimiter(1), 4, time.Second, func(frames [][]byte) ([][]float32, error) {
		sizes = append(sizes, len(frames))
		out := make([][]float32, len(frames))
		for i, frame := range frames {
			out[i] = []float32{float32(frame[0])}
		}
		return out, nil
	})
	frames := make([][]byte, 15)
	for i := range frames {
		frames[i] = []byte{byte(i)}
	}
	vectors, err := b.Embed(ctx, frames)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	b.Wait()
	for i, vector := range vectors {
		if vector[0] != float32(i) {
			t.Fatal("lost frame ordering")
		}
	}
	if !reflect.DeepEqual(sizes, []int{1, 2, 3, 4, 4, 1}) {
		t.Fatalf("unexpected batch ramp: %v", sizes)
	}
	if timings.Frames != 15 || timings.Batches != 6 || timings.MaxBatch != 4 {
		t.Fatalf("batch timing attribution counted frames twice: %+v", timings)
	}

}

func TestBatcherCancellationBehindSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	limiter := NewInferenceLimiter(1)
	release, err := limiter.Acquire(ctx, InteractiveInference)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	b := NewFrameBatcher(ctx, limiter, 2, time.Second, func([][]byte) ([][]float32, error) { t.Error("cancelled work ran"); return nil, nil })
	jobCtx, stopJob := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { _, err := b.Embed(jobCtx, [][]byte{{1}, {2}, {3}, {4}}); done <- err }()
	stopJob()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("cancelled job remained blocked")
	}
	cancel()
	b.Wait()
}

func TestBatcherShrinksSlowBatchesAndReportsInvalidOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var sizes []int
	b := NewFrameBatcher(ctx, NewInferenceLimiter(1), 4, 20*time.Millisecond, func(frames [][]byte) ([][]float32, error) {
		sizes = append(sizes, len(frames))
		if len(frames) > 1 {
			time.Sleep(25 * time.Millisecond)
		}
		return make([][]float32, len(frames)), nil
	})
	if _, err := b.Embed(ctx, [][]byte{{1}, {2}, {3}, {4}, {5}, {6}}); err != nil {
		t.Fatal(err)
	}
	cancel()
	b.Wait()
	if !reflect.DeepEqual(sizes, []int{1, 2, 1, 2}) {
		t.Fatalf("slow batches did not shrink: %v", sizes)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	bad := NewFrameBatcher(ctx2, NewInferenceLimiter(1), 1, time.Second, func([][]byte) ([][]float32, error) { return nil, nil })
	if _, err := bad.Embed(ctx2, [][]byte{{1}}); err == nil {
		t.Fatal("missing output accepted")
	}
	cancel2()
	bad.Wait()
}
