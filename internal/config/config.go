package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	PostgresDSN            string
	MediaRoot              string
	IngestConcurrency      int
	EmbeddingConcurrency   int
	InferenceConcurrency   int
	DecodeConcurrency      int
	EmbeddingBatchSize     int
	EmbeddingBatchTargetMS int
	DevMode                bool // enable development features like auto migration
}

func Load() (*Config, error) {
	// Load .env file if present; ignore error if not found
	_ = godotenv.Load()

	ingestConcurrency, err := positiveIntEnv("INGEST_CONCURRENCY", 2)
	if err != nil {
		return nil, err
	}
	embeddingConcurrency, err := positiveIntEnv("EMBEDDING_CONCURRENCY", 2)
	if err != nil {
		return nil, err
	}
	inferenceConcurrency, err := positiveIntEnv("INFERENCE_CONCURRENCY", 1)
	if err != nil {
		return nil, err
	}
	decodeConcurrency, err := positiveIntEnv("DECODE_CONCURRENCY", 1)
	if err != nil {
		return nil, err
	}
	batchSize, err := positiveIntEnv("EMBEDDING_BATCH_SIZE", 4)
	if err != nil {
		return nil, err
	}
	batchTarget, err := positiveIntEnv("EMBEDDING_BATCH_TARGET_MS", 150)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"ONNX_INTRA_THREADS", "VIPS_CONCURRENCY", "FFMPEG_THREADS"} {
		if os.Getenv(name) != "" {
			if _, err := positiveIntEnv(name, 1); err != nil {
				return nil, err
			}
		}
	}
	postgresDSN := os.Getenv("POSTGRES_DSN")
	if postgresDSN == "" {
		return nil, errors.New("POSTGRES_DSN is required")
	}
	cfg := &Config{
		PostgresDSN:            postgresDSN,
		MediaRoot:              getEnvOrDefault("MEDIA_ROOT", "/data/media"),
		IngestConcurrency:      ingestConcurrency,
		EmbeddingConcurrency:   embeddingConcurrency,
		InferenceConcurrency:   inferenceConcurrency,
		DecodeConcurrency:      decodeConcurrency,
		EmbeddingBatchSize:     batchSize,
		EmbeddingBatchTargetMS: batchTarget,
		DevMode:                getEnvOrDefault("DEV_MODE", "false") == "true",
	}
	return cfg, nil
}

func positiveIntEnv(key string, fallback int) (int, error) {
	value, err := strconv.Atoi(getEnvOrDefault(key, strconv.Itoa(fallback)))
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	return v
}
