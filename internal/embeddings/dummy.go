//go:build !embeddings

package embed

import (
	"context"
)

// Dummy stubs for vet/test without embeddings tag.

func VisionEmbedding(any interface{}) ([]float32, error) {
	return nil, nil
}

func TextEmbedding(any interface{}) ([]float32, error) {
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

// Add any types referenced in main.go or workers
type ModelOptions struct{}
