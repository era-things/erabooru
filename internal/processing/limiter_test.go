package processing

import (
	"context"
	"errors"
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
