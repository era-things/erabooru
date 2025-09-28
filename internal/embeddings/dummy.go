//go:build !embeddings

package embed

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

type ModelOptions struct{}
