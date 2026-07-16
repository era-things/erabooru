package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	PostgresDSN      string
	MediaRoot        string
	MediaConcurrency int
	DevMode          bool // enable development features like auto migration
}

func Load() (*Config, error) {
	// Load .env file if present; ignore error if not found
	_ = godotenv.Load()

	concurrency, err := strconv.Atoi(getEnvOrDefault("MEDIA_CONCURRENCY", "2"))
	if err != nil || concurrency < 1 {
		return nil, fmt.Errorf("MEDIA_CONCURRENCY must be a positive integer")
	}
	cfg := &Config{
		PostgresDSN:      getEnv("POSTGRES_DSN"),
		MediaRoot:        getEnvOrDefault("MEDIA_ROOT", "/data/media"),
		MediaConcurrency: concurrency,
		DevMode:          getEnv("DEV_MODE") == "true",
	}
	return cfg, nil
}

func getEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return v
}

func getEnvOrDefault(key, defaultValue string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	return v
}
