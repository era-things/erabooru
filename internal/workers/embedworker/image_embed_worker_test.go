package embedworker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
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
		batcher := processing.NewFrameBatcher(ctx, limiter, 2, time.Second, infer)
		vectors, err := batcher.Embed(ctx, [][]byte{{1}, {2}})
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
	batcher := processing.NewFrameBatcher(ctx, limiter, 2, time.Second, func([][]byte) ([][]float32, error) {
		return nil, want
	})
	_, err := batcher.Embed(ctx, [][]byte{{1}})
	if !errors.Is(err, want) {
		t.Fatalf("expected inference error, got %v", err)
	}
	release, err := limiter.Acquire(ctx, processing.InteractiveInference)
	if err != nil {
		t.Fatal("failed frame retained the inference slot:", err)
	}
	release()
}

func TestVideoSamplingCommands(t *testing.T) {
	timestamp := 30.0
	args := videoFrameArgs("input.mp4", 120, 12, 224, &timestamp)
	if slices.Index(args, "-ss") >= slices.Index(args, "-i") {
		t.Fatal("seek must precede input")
	}
	if args[slices.Index(args, "-frames:v")+1] != "1" {
		t.Fatal("seek must extract one frame")
	}
	if videoSamplingMode(59) != "sequential" || videoSamplingMode(60) != "seek" {
		t.Fatal("unexpected strategy")
	}
}

func TestExtractVideoFramesWithFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	src := filepath.Join(t.TempDir(), "sample.mp4")
	if out, err := exec.CommandContext(ctx, "ffmpeg", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=red:s=32x32:r=2:d=61", "-c:v", "mpeg4", "-g", "2", src).CombinedOutput(); err != nil {
		t.Fatalf("generate clip: %v: %s", err, out)
	}
	for _, duration := range []int{4, 60} {
		frames, err := extractVideoFrames(ctx, src, duration, 5, 16)
		if err != nil {
			t.Fatal(err)
		}
		if len(frames) != 5 {
			t.Fatalf("duration %d: got %d frames", duration, len(frames))
		}
		for _, frame := range frames {
			if len(frame) != 3*16*16 || frame[0] < 200 || frame[1] > 30 {
				t.Fatal("unexpected frame dimensions/color")
			}
		}
	}
}

func TestExtractVideoFramesIgnoresLargerShortSecondaryTrack(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	src := filepath.Join(t.TempDir(), "multiple-tracks.mkv")
	// Both tracks are default, but the larger blue track ends before every
	// sparse sample. Automatic selection silently produces zero frames.
	args := []string{"-loglevel", "error", "-f", "lavfi", "-i", "color=c=red:s=32x32:r=2:d=61",
		"-f", "lavfi", "-i", "color=c=blue:s=34x34:r=2:d=0.5",
		"-map", "0:v", "-map", "1:v", "-c:v", "ffv1",
		"-disposition:v:0", "default", "-disposition:v:1", "default", src}
	if out, err := exec.CommandContext(ctx, "ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("generate clip: %v: %s", err, out)
	}
	for _, duration := range []int{4, 60} {
		frames, err := extractVideoFrames(ctx, src, duration, 5, 16)
		if err != nil || len(frames) != 5 {
			t.Fatalf("duration %d: got %d frames, error %v", duration, len(frames), err)
		}
		for _, frame := range frames {
			if frame[0] < 200 || frame[2] > 30 {
				t.Fatal("selected secondary video track")
			}
		}
	}
}

func TestExtractVideoFramesFromExternalFixtures(t *testing.T) {
	paths := filepath.SplitList(os.Getenv("TEST_VIDEO_FIXTURES"))
	if len(paths) == 0 {
		t.Skip("set TEST_VIDEO_FIXTURES to video paths to verify without copying into the repository")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, src := range paths {
		out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", src).Output()
		if err != nil {
			t.Fatal(err)
		}
		seconds, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		if err != nil {
			t.Fatal(err)
		}
		duration := int(seconds + 0.5)
		samples := videoSampleCount(duration)
		frames, err := extractVideoFrames(ctx, src, duration, samples, 32)
		if err != nil || len(frames) != samples {
			t.Fatalf("%s: got %d/%d frames, error %v", src, len(frames), samples, err)
		}
		t.Logf("%s: extracted %d frames", filepath.Base(src), len(frames))
	}
}
