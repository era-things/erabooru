//go:build !embeddings

package embed

import (
	"context"
)

// Dummy stubs for vet/test without embeddings tag.

func VisionEmbedding([]byte) ([]float32, error) {
	return nil, nil
}

func VisionEmbeddingRGB24([]byte) ([]float32, error) {
	return nil, nil
}

func VisionEmbeddingsRGB24([][]byte) ([][]float32, error) {
	return nil, nil
}

func TextEmbedding(string) ([]float32, error) {
	return nil, nil
}

func DefaultModelOptionsFromEnv() ModelOptions {
	return ModelOptions{}
}

func EnsureModel(ctx context.Context, opts ModelOptions) (string, error) {
	return "", nil
}

func Load(dir string) error {
	return nil
}

func InputSpatialSize() int { return 0 }

// Add any types referenced in main.go or workers
type ModelOptions struct{}

func Available() bool { return false }
