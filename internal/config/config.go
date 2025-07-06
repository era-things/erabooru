package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	PostgresDSN           string
	MinioUser             string
	MinioPassword         string
	MinioBucket           string
	PreviewBucket         string
	MinioInternalEndpoint string // for SDK connection (e.g., "minio:9000")
	MinioPublicHost       string // for browser-facing host (e.g., "localhost")
	MinioPublicPrefix     string // e.g., "/minio"
	BlevePath             string // path to Bleve index, e.g., "/data/bleve"
	MinioSSL              bool
	DevMode               bool // enable development features like auto migration
}

func Load() (*Config, error) {
	// Load .env file if present; ignore error if not found
	_ = godotenv.Load()

	cfg := &Config{
		PostgresDSN:           getEnv("POSTGRES_DSN"),
		MinioUser:             getEnv("MINIO_ROOT_USER"),
		MinioPassword:         getEnv("MINIO_ROOT_PASSWORD"),
		MinioBucket:           getEnv("MINIO_BUCKET"),
		PreviewBucket:         getEnv("MINIO_PREVIEW_BUCKET"),
		MinioInternalEndpoint: getEnv("MINIO_INTERNAL_ENDPOINT"),        // for SDK connection
		MinioPublicHost:       getEnvOrDefault("MINIO_PUBLIC_HOST", ""), // for browser-facing host
		MinioPublicPrefix:     getEnv("MINIO_PUBLIC_PREFIX"),            // e.g., "/minio" for Caddy reverse proxy
		BlevePath:             getEnv("BLEVE_PATH"),
		MinioSSL:              getEnv("MINIO_SSL") == "true",
		DevMode:               getEnv("DEV_MODE") == "true",
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
