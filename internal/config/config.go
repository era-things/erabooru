package config

import (
	"fmt"
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
	PreviewBucket         string
	MinioInternalEndpoint string // for SDK connection (e.g., "minio:9000")
	MinioPublicHost       string // for browser-facing host (e.g., "localhost")
	MinioPublicPrefix     string // e.g., "/minio"
	BlevePath             string // path to Bleve index, e.g., "/data/bleve"
	MinioSSL              bool
	VideoWorkerURL        string // address of the video worker service
	DevMode               bool   // enable development features like auto migration
}

func Load() (*Config, error) {
	// Load .env file if present; ignore error if not found
	_ = godotenv.Load()

	cfg := &Config{
		DBHost:                getEnv("POSTGRES_HOST"),
		DBPort:                getEnv("POSTGRES_PORT"),
		DBUser:                getEnv("POSTGRES_USER"),
		DBPassword:            getEnv("POSTGRES_PASSWORD"),
		DBName:                getEnv("POSTGRES_DB"),
		MinioUser:             getEnv("MINIO_ROOT_USER"),
		MinioPassword:         getEnv("MINIO_ROOT_PASSWORD"),
		MinioBucket:           getEnv("MINIO_BUCKET"),
		PreviewBucket:         getEnv("MINIO_PREVIEW_BUCKET"),
		MinioInternalEndpoint: getEnv("MINIO_INTERNAL_ENDPOINT"), // for SDK connection
		MinioPublicHost:       getEnv("MINIO_PUBLIC_HOST"),       // for browser-facing host
		MinioPublicPrefix:     getEnv("MINIO_PUBLIC_PREFIX"),     // e.g., "/minio" for Caddy reverse proxy
		BlevePath:             getEnv("BLEVE_PATH"),
		MinioSSL:              getEnv("MINIO_SSL") == "true",
		VideoWorkerURL:        getEnv("VIDEO_WORKER_URL"),
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
