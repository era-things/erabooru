package config

import "testing"

func TestThroughputConfiguration(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://test")
	for _, name := range []string{"DECODE_CONCURRENCY", "EMBEDDING_BATCH_SIZE", "EMBEDDING_BATCH_TARGET_MS", "ONNX_INTRA_THREADS", "VIPS_CONCURRENCY", "FFMPEG_THREADS"} {
		t.Setenv(name, "2")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DecodeConcurrency != 2 || cfg.EmbeddingBatchSize != 2 || cfg.EmbeddingBatchTargetMS != 2 {
		t.Fatalf("settings not loaded: %+v", cfg)
	}
	for _, name := range []string{"DECODE_CONCURRENCY", "EMBEDDING_BATCH_SIZE", "EMBEDDING_BATCH_TARGET_MS", "ONNX_INTRA_THREADS", "VIPS_CONCURRENCY", "FFMPEG_THREADS"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "0")
			if _, err := Load(); err == nil {
				t.Fatalf("accepted invalid %s", name)
			}
		})
	}
}
