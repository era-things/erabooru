package common

import (
	"os"
	"testing"

	"era/booru/internal/config"
)

// SetupEnv configures environment variables for integration tests and returns the loaded config.
func SetupEnv(t testing.TB, dsn string) *config.Config {
	t.Helper()

	os.Setenv("POSTGRES_DSN", dsn)
	os.Setenv("MEDIA_ROOT", t.TempDir())
	os.Setenv("DEV_MODE", "true")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}
