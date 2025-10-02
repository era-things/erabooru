package config

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// VideoHWAccelProfile describes the ffmpeg command-line knobs required for a
// supported hardware decoder.
type VideoHWAccelProfile struct {
	// HWAccelFlag is the value passed to -hwaccel.
	HWAccelFlag string
	// DefaultOutputFormat is used for -hwaccel_output_format when the user
	// does not override VIDEO_HW_OUTPUT_FORMAT.
	DefaultOutputFormat string
	// DeviceFlags enumerates additional flags that should be paired with
	// VIDEO_HW_DEVICE (e.g. -hwaccel_device, -vaapi_device).
	DeviceFlags []string
}

// SupportedVideoHWAccel enumerates the hardware acceleration profiles that the
// application knows how to wire into ffmpeg.
var SupportedVideoHWAccel = map[string]VideoHWAccelProfile{
	"cuda": {
		HWAccelFlag:         "cuda",
		DefaultOutputFormat: "cuda",
		DeviceFlags:         []string{"-hwaccel_device"},
	},
}

// SupportedVideoHWAccelOrder controls the preference order used when selecting a
// hardware acceleration profile automatically. The first supported value that
// is detected on the host will be selected.
var SupportedVideoHWAccelOrder = []string{"cuda"}

var (
	autoDetectOnce sync.Once
	autoDetectHW   string
	autoDetectErr  error
)

var hwAccelProbe = defaultHWAccelProbe

// FFmpegHWAccelArgs resolves VIDEO_HWACCEL* settings into ffmpeg arguments. The
// returned slice should be prepended to the ffmpeg command. When no hardware
// acceleration is requested, an empty slice is returned.
func FFmpegHWAccelArgs(cfg *Config) ([]string, error) {
	if cfg == nil {
		return nil, nil
	}

	if cfg.VideoHWAccelDisable {
		return nil, nil
	}

	accel := strings.TrimSpace(strings.ToLower(cfg.VideoHWAccel))
	if accel == "" {
		detected, err := getAutoDetectedHWAccel()
		if err == nil {
			accel = detected
		}
	}

	if accel == "" {
		return nil, nil
	}

	profile, ok := SupportedVideoHWAccel[accel]
	if !ok {
		return nil, fmt.Errorf("unsupported VIDEO_HWACCEL value %q", cfg.VideoHWAccel)
	}

	args := []string{"-hwaccel", profile.HWAccelFlag}

	outputFormat := strings.TrimSpace(cfg.VideoHWOutputFormat)
	if outputFormat == "" {
		outputFormat = profile.DefaultOutputFormat
	}
	if outputFormat != "" {
		args = append(args, "-hwaccel_output_format", outputFormat)
	}

	device := strings.TrimSpace(cfg.VideoHWDevice)
	if device != "" {
		for _, flag := range profile.DeviceFlags {
			args = append(args, flag, device)
		}
	}

	return args, nil
}

func getAutoDetectedHWAccel() (string, error) {
	autoDetectOnce.Do(func() {
		autoDetectHW, autoDetectErr = detectAvailableHWAccel()
		if autoDetectErr != nil {
			autoDetectHW = ""
		}
	})
	return autoDetectHW, autoDetectErr
}

func detectAvailableHWAccel() (string, error) {
	names, err := hwAccelProbe()
	if err != nil {
		return "", err
	}

	available := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" {
			continue
		}
		available[name] = struct{}{}
	}

	for _, candidate := range SupportedVideoHWAccelOrder {
		if _, ok := available[candidate]; ok {
			return candidate, nil
		}
	}

	return "", nil
}

func defaultHWAccelProbe() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-hwaccels")
	output, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	var names []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, ":") {
			continue
		}
		names = append(names, line)
	}

	return names, nil
}
