package embedworker

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"era/booru/internal/processing"
)

func TestVideoSampleCountIsBounded(t *testing.T) {
	tests := []struct {
		duration int
		want     int
	}{
		{duration: 3, want: 5},
		{duration: 30, want: 8},
		{duration: 120, want: 12},
		{duration: 3600, want: 16},
	}
	for _, test := range tests {
		if got := videoSampleCount(test.duration); got != test.want {
			t.Errorf("duration %d: expected %d samples, got %d", test.duration, test.want, got)
		}
	}
}

func TestVideoFramesYieldToSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	limiter := processing.NewInferenceLimiter(2)
	searchRunning := make(chan struct{})
	finishSearch := make(chan struct{})
	secondFrame := make(chan struct{})
	completed := make(chan error, 1)
	infer := func(frames [][]byte) ([][]float32, error) {
		if len(frames) != 1 {
			return nil, fmt.Errorf("expected one frame per admission, got %d", len(frames))
		}
		if frames[0][0] == 1 {
			go func() {
				release, err := limiter.Acquire(ctx, processing.InteractiveInference)
				if err != nil {
					return
				}
				defer release()
				close(searchRunning)
				select {
				case <-finishSearch:
				case <-ctx.Done():
				}
			}()
			select {
			case <-searchRunning:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		} else {
			close(secondFrame)
		}
		return [][]float32{{float32(frames[0][0]), 1}}, nil
	}
	go func() {
		vectors, err := embedVideoFrames(ctx, limiter, [][]byte{{1}, {2}}, infer)
		if err == nil && !reflect.DeepEqual(vectors, [][]float32{{1, 1}, {2, 1}}) {
			err = fmt.Errorf("frame vectors changed: %v", vectors)
		}
		completed <- err
	}()
	select {
	case <-searchRunning:
	case <-ctx.Done():
		t.Fatal("search did not start in reserved slot")
	}
	select {
	case <-secondFrame:
		t.Fatal("video advanced while search was active")
	default:
	}
	close(finishSearch)
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("video did not resume after search")
	}
}

func TestVideoFrameFailureReleasesInference(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	limiter := processing.NewInferenceLimiter(1)
	want := errors.New("inference failed")
	_, err := embedVideoFrames(ctx, limiter, [][]byte{{1}}, func([][]byte) ([][]float32, error) {
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected inference error, got %v", err)
	}
	release, err := limiter.Acquire(ctx, processing.InteractiveInference)
	if err != nil {
		t.Fatal("failed frame retained the inference slot:", err)
	}
	release()
}
