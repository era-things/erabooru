package common

import (
	"os"
	"path/filepath"
	"testing"

	"era/booru/internal/config"
)

// SetupEnv configures environment variables for integration tests and returns the loaded config.
func SetupEnv(t testing.TB, dsn, minioAddr string) *config.Config {
	t.Helper()

	os.Setenv("POSTGRES_DSN", dsn)
	os.Setenv("MINIO_ROOT_USER", "minioadmin")
	os.Setenv("MINIO_ROOT_PASSWORD", "minio123")
	os.Setenv("MINIO_BUCKET", "boorubucket")
	os.Setenv("MINIO_PREVIEW_BUCKET", "previews")
	os.Setenv("MINIO_INTERNAL_ENDPOINT", minioAddr)
	os.Setenv("MINIO_PUBLIC_HOST", "")
	os.Setenv("MINIO_PUBLIC_PREFIX", "boorubucket")
	os.Setenv("MINIO_SSL", "false")
	os.Setenv("DEV_MODE", "true")
	os.Setenv("VIDEO_HWACCEL", "")
	os.Setenv("VIDEO_HW_OUTPUT_FORMAT", "")
	os.Setenv("VIDEO_HW_DEVICE", "")
	os.Setenv("VIDEO_HWACCEL_DISABLE", "")
	bleveDir := filepath.Join(t.TempDir(), "bleve")
	os.Setenv("BLEVE_PATH", bleveDir)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}
