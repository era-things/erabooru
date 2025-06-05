package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost                string
	DBPort                string
	DBUser                string
	DBPassword            string
	DBName                string
	MinioUser             string
	MinioPassword         string
	MinioBucket           string
	MinioInternalEndpoint string // for SDK connection (e.g., "minio:9000")
	MinioPublicHost       string // for browser-facing host (e.g., "localhost")
	MinioPublicPrefix     string // e.g., "/minio"
	MinioSSL              bool
}

func Load() (*Config, error) {
	// Load .env file if present; ignore error if not found
	_ = godotenv.Load()

	cfg := &Config{
		DBHost:                getEnv("POSTGRES_HOST", "localhost"),
		DBPort:                getEnv("POSTGRES_PORT", "5432"),
		DBUser:                getEnv("POSTGRES_USER", "booru"),
		DBPassword:            getEnv("POSTGRES_PASSWORD", "booru"),
		DBName:                getEnv("POSTGRES_DB", "booru"),
		MinioUser:             getEnv("MINIO_ROOT_USER", "minio"),
		MinioPassword:         getEnv("MINIO_ROOT_PASSWORD", "minio123"),
		MinioBucket:           getEnv("MINIO_BUCKET", "boorubucket"),
		MinioInternalEndpoint: getEnv("MINIO_INTERNAL_ENDPOINT", "minio:9000"), // for SDK connection
		MinioPublicHost:       getEnv("MINIO_PUBLIC_HOST", "localhost:9000"),   // for browser-facing host
		MinioPublicPrefix:     getEnv("MINIO_PUBLIC_PREFIX", "/minio"),         // e.g., "/minio" for Caddy reverse proxy
		MinioSSL:              getEnv("MINIO_SSL", "false") == "true",
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
