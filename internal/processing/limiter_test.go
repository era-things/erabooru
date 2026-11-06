package processing

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

func TestLimiterHonorsContext(t *testing.T) {
	limiter := NewLimiter(1)
	release, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := limiter.Acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestInferenceLimiterPrioritizesInteractiveWork(t *testing.T) {
	limiter := NewInferenceLimiter(1)
	release, err := limiter.Acquire(context.Background(), BackgroundInference)
	if err != nil {
		t.Fatal(err)
	}

	order := make(chan InferencePriority, 2)
	for _, priority := range []InferencePriority{BackgroundInference, InteractiveInference} {
		go func(priority InferencePriority) {
			release, err := limiter.Acquire(context.Background(), priority)
			if err != nil {
				return
			}
			order <- priority
			release()
		}(priority)
	}

	waitForInferenceQueues(t, limiter, 1, 1)
	release()

	if first := <-order; first != InteractiveInference {
		t.Fatalf("expected interactive inference first, got %v", first)
	}
	if second := <-order; second != BackgroundInference {
		t.Fatalf("expected background inference second, got %v", second)
	}
}

func waitForInferenceQueues(t *testing.T, limiter *InferenceLimiter, background, interactive int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		limiter.mu.Lock()
		bg, fg := limiter.background.Len(), limiter.interactive.Len()
		limiter.mu.Unlock()
		if bg == background && fg == interactive {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiting queues: background=%d interactive=%d, want %d/%d", bg, fg, background, interactive)
		}
		runtime.Gosched()
	}
}

func TestInferenceLimiterReservesSearchSlotAndPausesBackground(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	limiter := NewInferenceLimiter(2)
	releaseBackground, err := limiter.Acquire(ctx, BackgroundInference)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseBackground()
	type admission struct {
		release func()
		err     error
	}
	background := make(chan admission, 1)
	go func() {
		release, err := limiter.Acquire(ctx, BackgroundInference)
		background <- admission{release, err}
	}()
	waitForInferenceQueues(t, limiter, 1, 0)

	// The waiting background job cannot consume the slot reserved for search.
	releaseSearch, err := limiter.Acquire(ctx, InteractiveInference)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseSearch()
	releaseBackground()
	limiter.mu.Lock()
	paused := limiter.activeBackground == 0 && limiter.background.Len() == 1
	limiter.mu.Unlock()
	if !paused {
		t.Fatal("background work started while search was active")
	}

	releaseSearch()
	select {
	case result := <-background:
		if result.err != nil {
			t.Fatal(result.err)
		}
		result.release()
	case <-ctx.Done():
		t.Fatal("background work did not resume after search")
	}
}

func TestInferenceLimiterCanceledWaiterDoesNotBlockWork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	limiter := NewInferenceLimiter(1)
	release, err := limiter.Acquire(ctx, BackgroundInference)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	searchCtx, cancelSearch := context.WithCancel(ctx)
	defer cancelSearch()
	result := make(chan error, 1)
	go func() {
		unlock, err := limiter.Acquire(searchCtx, InteractiveInference)
		if unlock != nil {
			unlock()
		}
		result <- err
	}()
	waitForInferenceQueues(t, limiter, 0, 1)
	cancelSearch()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	case <-ctx.Done():
		t.Fatal("canceled search remained blocked")
	}
	release()
	releaseNext, err := limiter.Acquire(ctx, BackgroundInference)
	if err != nil {
		t.Fatal(err)
	}
	releaseNext()
}

func TestInferenceLimiterRejectsAlreadyCanceledSearch(t *testing.T) {
	limiter := NewInferenceLimiter(2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := limiter.Acquire(ctx, InteractiveInference); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation on an idle limiter, got %v", err)
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if limiter.activeInteractive != 0 || limiter.interactive.Len() != 0 {
		t.Fatal("canceled search retained an inference slot")
	}
}

func TestInferenceLimiterDrainsSearchesBeforeBackground(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	limiter := NewInferenceLimiter(1)
	release, err := limiter.Acquire(ctx, BackgroundInference)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	type admission struct {
		index   int
		release func()
		err     error
	}
	admitted := make(chan admission, 4)
	go func() {
		unlock, err := limiter.Acquire(ctx, BackgroundInference)
		admitted <- admission{-1, unlock, err}
	}()
	waitForInferenceQueues(t, limiter, 1, 0)
	for i := range 3 {
		go func() {
			unlock, err := limiter.Acquire(ctx, InteractiveInference)
			admitted <- admission{i, unlock, err}
		}()
		waitForInferenceQueues(t, limiter, 1, i+1)
	}
	release()
	for _, want := range []int{0, 1, 2, -1} {
		select {
		case result := <-admitted:
			if result.err != nil {
				t.Fatal(result.err)
			}
			if result.index != want {
				result.release()
				t.Fatalf("admitted %d, want %d", result.index, want)
			}
			result.release()
		case <-ctx.Done():
			t.Fatal("inference queue stopped progressing")
		}
	}
}
