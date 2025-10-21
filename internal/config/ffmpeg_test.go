package config

import (
	"errors"
	"sync"
	"testing"
)

func TestFFmpegHWAccelArgsDisabled(t *testing.T) {
	cfg := &Config{}
	args, err := FFmpegHWAccelArgs(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args when accel disabled, got %v", args)
	}
}

func TestFFmpegHWAccelArgsExplicitDisable(t *testing.T) {
	cfg := &Config{VideoHWAccelDisable: true, VideoHWAccel: "cuda"}
	args, err := FFmpegHWAccelArgs(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args when accel explicitly disabled, got %v", args)
	}
}

func TestFFmpegHWAccelArgsCUDA(t *testing.T) {
	cfg := &Config{VideoHWAccel: "CUDA"}
	args, err := FFmpegHWAccelArgs(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d (%v)", len(expected), len(args), args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("arg %d: expected %q, got %q (full=%v)", i, expected[i], args[i], args)
		}
	}
}

func TestFFmpegHWAccelArgsCUDADeviceAndOverride(t *testing.T) {
	cfg := &Config{VideoHWAccel: "cuda", VideoHWDevice: "0", VideoHWOutputFormat: "nv12"}
	args, err := FFmpegHWAccelArgs(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		"-hwaccel", "cuda",
		"-hwaccel_output_format", "nv12",
		"-hwaccel_device", "0",
	}

	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d (%v)", len(expected), len(args), args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("arg %d: expected %q, got %q (full=%v)", i, expected[i], args[i], args)
		}
	}
}

func TestFFmpegHWAccelArgsUnsupported(t *testing.T) {
	cfg := &Config{VideoHWAccel: "magic"}
	if _, err := FFmpegHWAccelArgs(cfg); err == nil {
		t.Fatalf("expected error for unsupported accel")
	}
}

func TestFFmpegHWAccelArgsAutoDetect(t *testing.T) {
	t.Cleanup(func() {
		hwAccelProbe = defaultHWAccelProbe
		autoDetectOnce = sync.Once{}
		autoDetectHW = ""
		autoDetectErr = nil
	})

	hwAccelProbe = func() ([]string, error) {
		return []string{"cuda"}, nil
	}
	autoDetectOnce = sync.Once{}
	autoDetectHW = ""
	autoDetectErr = nil

	cfg := &Config{}
	args, err := FFmpegHWAccelArgs(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d (%v)", len(expected), len(args), args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("arg %d: expected %q, got %q (full=%v)", i, expected[i], args[i], args)
		}
	}
}

func TestFFmpegHWAccelArgsAutoDetectErrorIgnored(t *testing.T) {
	t.Cleanup(func() {
		hwAccelProbe = defaultHWAccelProbe
		autoDetectOnce = sync.Once{}
		autoDetectHW = ""
		autoDetectErr = nil
	})

	hwAccelProbe = func() ([]string, error) {
		return nil, errors.New("boom")
	}
	autoDetectOnce = sync.Once{}
	autoDetectHW = ""
	autoDetectErr = nil

	cfg := &Config{}
	args, err := FFmpegHWAccelArgs(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args when detection fails, got %v", args)
	}
}
