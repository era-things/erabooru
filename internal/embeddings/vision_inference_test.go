//go:build embeddings

package embed

import (
	"os"
	"testing"
)

// Opt-in: exercises the actual cached model without downloading weights.
func TestNativeVisionBatchMatchesIndividual(t *testing.T) {
	dir := os.Getenv("MODEL_TEST_DIR")
	if dir == "" {
		t.Skip("set MODEL_TEST_DIR to cached SigLIP model directory")
	}
	if err := Load(dir); err != nil {
		t.Fatal(err)
	}
	size := InputSpatialSize()
	frames := make([][]byte, 4)
	for i := range frames {
		frames[i] = make([]byte, 3*size*size)
		for pixel := 0; pixel < size*size; pixel++ {
			frames[i][pixel*3] = byte((pixel + i*31) % 256)
			frames[i][pixel*3+1] = byte((pixel/size + i*53) % 256)
			frames[i][pixel*3+2] = byte(i * 47)
		}
	}
	batch, err := VisionEmbeddingsRGB24(frames)
	if err != nil {
		t.Fatal(err)
	}
	for i, frame := range frames {
		single, err := VisionEmbeddingsRGB24([][]byte{frame})
		if err != nil {
			t.Fatal(err)
		}
		if len(batch[i]) != 768 || len(single[0]) != 768 {
			t.Fatal("unexpected embedding dimension")
		}
		var cosine float32
		for j, v := range batch[i] {
			cosine += v * single[0][j]
		}
		if cosine < 0.999 {
			t.Fatalf("frame %d changed in batch: cosine=%f", i, cosine)
		}
	}
}
