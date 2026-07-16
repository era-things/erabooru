package processing

import (
	"context"
	"errors"
	"runtime"
	"testing"
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

	for {
		limiter.mu.Lock()
		waiting := limiter.background.Len() + limiter.interactive.Len()
		limiter.mu.Unlock()
		if waiting == 2 {
			break
		}
		runtime.Gosched()
	}
	release()

	if first := <-order; first != InteractiveInference {
		t.Fatalf("expected interactive inference first, got %v", first)
	}
	if second := <-order; second != BackgroundInference {
		t.Fatalf("expected background inference second, got %v", second)
	}
}
