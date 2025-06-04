package config

import (
	"github.com/joho/godotenv"
	"os"
)

type Config struct {
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	MinioEndpoint string
	MinioUser     string
	MinioPassword string
	MinioBucket   string
	MinioSSL      bool
}

func Load() (*Config, error) {
	// Load .env file if present; ignore error if not found
	_ = godotenv.Load()

	cfg := &Config{
		DBHost:        getEnv("POSTGRES_HOST", "localhost"),
		DBPort:        getEnv("POSTGRES_PORT", "5432"),
		DBUser:        getEnv("POSTGRES_USER", "booru"),
		DBPassword:    getEnv("POSTGRES_PASSWORD", "booru"),
		DBName:        getEnv("POSTGRES_DB", "booru"),
		MinioEndpoint: getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioUser:     getEnv("MINIO_ROOT_USER", "minio"),
		MinioPassword: getEnv("MINIO_ROOT_PASSWORD", "minio123"),
		MinioBucket:   getEnv("MINIO_BUCKET", "boorubucket"),
		MinioSSL:      getEnv("MINIO_SSL", "false") == "true",
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
