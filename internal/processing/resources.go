package processing

import (
	"os"
	"runtime"
	"strconv"
)

// ResourceThreads leaves CPU capacity for decoding, HTTP requests and Postgres.
// Explicit positive overrides take precedence; config.Load validates them.
func ResourceThreads(name string, fallback int) int {
	if n, err := strconv.Atoi(os.Getenv(name)); err == nil && n > 0 {
		return n
	}
	return max(1, fallback)
}

func DefaultInferenceThreads() int {
	return max(1, min(3, (runtime.GOMAXPROCS(0)-1)/ResourceThreads("INFERENCE_CONCURRENCY", 1)))
}

func FFmpegThreads() string { return strconv.Itoa(ResourceThreads("FFMPEG_THREADS", 1)) }
