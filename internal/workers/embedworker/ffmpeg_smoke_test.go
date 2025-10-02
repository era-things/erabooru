package embedworker

import (
	"bytes"
	"os/exec"
	"testing"
)

// TestFFmpegBinaryAvailable exercises a tiny ffmpeg pipeline without hardware
// acceleration enabled. This acts as a smoke test to ensure that the default
// configuration keeps functioning when VIDEO_HWACCEL is unset.
func TestFFmpegBinaryAvailable(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed: " + err.Error())
	}

	cmd := exec.Command("ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "lavfi",
		"-i", "color=c=black:s=8x8:d=0.05",
		"-frames:v", "1",
		"-pix_fmt", "rgb24",
		"-f", "rawvideo",
		"-",
	)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("ffmpeg command failed: %v (stderr=%s)", err, stderr.String())
	}

	if out.Len() == 0 {
		t.Fatalf("expected ffmpeg to emit raw pixels")
	}
}
