package processing

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFFmpegRepairsReservedMetadata(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, metadata := range []string{
		"colour_primaries=3:transfer_characteristics=3:matrix_coefficients=3",
		"colour_primaries=1:transfer_characteristics=1:matrix_coefficients=1:video_full_range_flag=1",
	} {
		src := filepath.Join(t.TempDir(), "color.mp4")
		args := []string{"-v", "error", "-f", "lavfi", "-i", "color=c=red:s=32x32:d=1", "-c:v", "libx264",
			"-bsf:v", "h264_metadata=" + metadata, src}
		if out, err := exec.CommandContext(ctx, "ffmpeg", args...).CombinedOutput(); err != nil {
			t.Fatalf("generate fixture: %v: %s", err, out)
		}
		decode := []string{"-v", "error", "-i", src, "-map", "0:v:0", "-frames:v", "1", "-vf", "scale=16:16", "-pix_fmt", "rgb24", "-f", "rawvideo", "pipe:1"}
		repair, err := colorMetadataRepair(ctx, src)
		if err != nil || (repair != "") != strings.Contains(metadata, "colour_primaries=3") {
			t.Fatalf("repair=%q err=%v", repair, err)
		}
		output, err := FFmpegOutput(ctx, decode...)
		if err != nil || len(output) != 3*16*16 {
			t.Fatalf("decoded %d bytes: %v", len(output), err)
		}
		if repair == "" {
			expected, err := ffmpegOutput(ctx, decode)
			if err != nil || !bytes.Equal(output, expected) {
				t.Fatalf("valid metadata changed: %v", err)
			}
		} else {
			// The fallback must only alter the input stream in memory.
			if after, err := colorMetadataRepair(ctx, src); err != nil || after != repair {
				t.Fatalf("source metadata changed: repair=%q err=%v", after, err)
			}
		}
	}
}
