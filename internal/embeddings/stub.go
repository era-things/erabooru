//go:build !embeddings

package embed

import "fmt"

func Load(string) error { return fmt.Errorf("embeddings support not built") }

func VisionEmbedding(any) ([]float32, error) {
	return nil, fmt.Errorf("embeddings support not built")
}

func TextEmbedding(string) ([]float32, error) {
	return nil, fmt.Errorf("embeddings support not built")
}
