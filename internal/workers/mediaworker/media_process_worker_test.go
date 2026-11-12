package mediaworker

import (
	"bytes"
	"context"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPreviewIgnoresLargerShortSecondaryTrack(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	src := filepath.Join(t.TempDir(), "multiple-tracks.mkv")
	args := []string{"-loglevel", "error", "-f", "lavfi", "-i", "color=c=red:s=32x32:r=2:d=4",
		"-f", "lavfi", "-i", "color=c=blue:s=34x34:r=2:d=0.5",
		"-map", "0:v", "-map", "1:v", "-c:v", "ffv1",
		"-disposition:v:0", "default", "-disposition:v:1", "default", src}
	if out, err := exec.CommandContext(ctx, "ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("generate clip: %v: %s", err, out)
	}
	for _, seek := range []string{"", "2.000"} {
		cmd := exec.CommandContext(ctx, "ffmpeg", previewArgs(src, seek)...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("preview at %q: %v: %s", seek, err, stderr.String())
		}
		img, err := jpeg.Decode(bytes.NewReader(output))
		if err != nil {
			t.Fatalf("preview at %q is not a JPEG: %v", seek, err)
		}
		r, _, b, _ := img.At(0, 0).RGBA()
		if r < 50000 || b > 10000 {
			t.Fatalf("preview at %q selected secondary track", seek)
		}
	}
}

func TestPreviewFromExternalFixtures(t *testing.T) {
	paths := filepath.SplitList(os.Getenv("TEST_VIDEO_FIXTURES"))
	if len(paths) == 0 {
		t.Skip("set TEST_VIDEO_FIXTURES to video paths")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, src := range paths {
		output, err := exec.CommandContext(ctx, "ffmpeg", previewArgs(src, "2.000")...).Output()
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if _, err := jpeg.Decode(bytes.NewReader(output)); err != nil {
			t.Fatalf("%s: invalid preview: %v", src, err)
		}
		t.Logf("%s: generated valid JPEG preview", filepath.Base(src))
	}
}
