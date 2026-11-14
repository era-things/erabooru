package processing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

// FFmpegOutput retries invalid color metadata using an input bitstream filter.
// Repairing after decoding is too late: FFmpeg rejects the filter graph first.
// Originals and valid color metadata are left untouched.
func FFmpegOutput(ctx context.Context, args ...string) ([]byte, error) {
	output, err := ffmpegOutput(ctx, args)
	input := slices.Index(args, "-i")
	if err == nil || input < 0 || input+1 >= len(args) ||
		(!strings.Contains(err.Error(), "Invalid color range") && !strings.Contains(err.Error(), "Invalid color space")) {
		return output, err
	}
	repair, probeErr := colorMetadataRepair(ctx, args[input+1])
	if probeErr != nil || repair == "" {
		return nil, err
	}
	retry := append(slices.Clone(args[:input]), "-bsf:v:0", repair)
	retry = append(retry, args[input:]...)
	return ffmpegOutput(ctx, retry)
}

func ffmpegOutput(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

func colorMetadataRepair(ctx context.Context, src string) (string, error) {
	output, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=codec_name,color_space,color_transfer,color_primaries", "-of", "json", src).Output()
	if err != nil {
		return "", err
	}
	var probe struct {
		Streams []map[string]string `json:"streams"`
	}
	if err := json.Unmarshal(output, &probe); err != nil {
		return "", err
	}
	if len(probe.Streams) == 0 {
		return "", nil
	}
	stream := probe.Streams[0]
	codec := stream["codec_name"]
	if codec != "h264" && codec != "hevc" {
		return "", nil
	}
	var fields []string
	for _, field := range [][2]string{{"color_space", "matrix_coefficients"}, {"color_transfer", "transfer_characteristics"}, {"color_primaries", "colour_primaries"}} {
		if stream[field[0]] == "reserved" {
			fields = append(fields, field[1]+"=2") // unspecified, rather than guessing BT.709
		}
	}
	if len(fields) == 0 {
		return "", nil
	}
	return codec + "_metadata=" + strings.Join(fields, ":"), nil
}
