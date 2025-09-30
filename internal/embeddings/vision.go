//go:build embeddings

package embed

import (
	"fmt"
	"image"
	"math"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	ort "github.com/yalue/onnxruntime_go"
)

// VisionEmbedding converts an image to an L2-normalised embedding vector.
func VisionEmbedding(img image.Image) ([]float32, error) {
	vecs, err := VisionEmbeddingBatch([]image.Image{img})
	if err != nil {
		return nil, err
	}
	if len(vecs) != 1 {
		return nil, fmt.Errorf("unexpected batch size %d", len(vecs))
	}
	return vecs[0], nil
}

// VisionEmbeddingBatch converts a batch of images to L2-normalised embedding vectors.
func VisionEmbeddingBatch(imgs []image.Image) ([][]float32, error) {
	if len(imgs) == 0 {
		return nil, fmt.Errorf("no images provided")
	}

	const maxAttempts = 3

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		S := InputSpatialSize()
		vecs, err := visionEmbeddingBatchWithSize(imgs, S)
		if err == nil {
			return vecs, nil
		}
		lastErr = err

		if newSize, ok := retargetVisionInputSize(err, S); ok {
			setInputSpatialSize(newSize)
			continue
		}

		return nil, err
	}

	return nil, lastErr
}

func l2(v []float32) {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return
	}
	scale := float32(1.0 / math.Sqrt(sum))
	for i := range v {
		v[i] *= scale
	}
}

func visionEmbeddingBatchWithSize(src []image.Image, S int) ([][]float32, error) {
	if S <= 0 {
		return nil, fmt.Errorf("invalid vision input size %d", S)
	}
	if len(src) == 0 {
		return nil, fmt.Errorf("no images provided")
	}

	batch := len(src)
	pix := make([]float32, batch*3*S*S)
	for b, imgSrc := range src {
		img := imaging.Fill(imgSrc, S, S, imaging.Center, imaging.Lanczos)
		base := b * 3 * S * S
		for y := 0; y < S; y++ {
			for x := 0; x < S; x++ {
				r, g, bcol, _ := img.At(x, y).RGBA()
				i := y*S + x
				pix[base+i] = (float32(r>>8)/255 - .5) / .5
				pix[base+S*S+i] = (float32(g>>8)/255 - .5) / .5
				pix[base+2*S*S+i] = (float32(bcol>>8)/255 - .5) / .5
			}
		}
	}

	in, err := ort.NewTensor[float32](ort.NewShape(int64(batch), 3, int64(S), int64(S)), pix)
	if err != nil {
		return nil, err
	}
	defer in.Destroy()

	outputs := []ort.Value{nil}
	if err := Session().Run(
		[]ort.Value{in},
		outputs,
	); err != nil {
		return nil, fmt.Errorf("failed to run ONNX model: %w", err)
	}

	out, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected output tensor type %T", outputs[0])
	}
	defer out.Destroy()

	data := out.GetData()
	shape := out.GetShape()

	if len(shape) < 2 {
		return nil, fmt.Errorf("unexpected embedding rank %d", len(shape))
	}
	if shape[0] != int64(batch) {
		return nil, fmt.Errorf("unexpected batch dimension %d", shape[0])
	}

	var dim int
	switch len(shape) {
	case 2:
		dim = int(shape[1])
	default:
		return nil, fmt.Errorf("unsupported embedding rank %d", len(shape))
	}

	expected := batch * dim
	if len(data) != expected {
		return nil, fmt.Errorf("embedding data length mismatch: got %d, expected %d", len(data), expected)
	}

	vecs := make([][]float32, batch)
	for i := 0; i < batch; i++ {
		start := i * dim
		vec := make([]float32, dim)
		copy(vec, data[start:start+dim])
		l2(vec)
		vecs[i] = vec
	}

	return vecs, nil
}

func retargetVisionInputSize(err error, current int) (int, bool) {
	msg := err.Error()
	inputDims, ok := parseShapeFromError(msg, "Input shape:{")
	if !ok {
		return 0, false
	}
	requestedDims, ok := parseShapeFromError(msg, "requested shape:{")
	if !ok {
		return 0, false
	}

	if len(inputDims) < 3 {
		return 0, false
	}
	spatial := inputDims[len(inputDims)-1]
	if spatial <= 0 {
		return 0, false
	}
	if current <= 0 || current%spatial != 0 {
		return 0, false
	}

	expectedTokens := requestedDims[len(requestedDims)-1]
	if expectedTokens <= 0 {
		return 0, false
	}
	expectedPerDim := int(math.Round(math.Sqrt(float64(expectedTokens))))
	if expectedPerDim*expectedPerDim != expectedTokens {
		return 0, false
	}

	patchSize := current / spatial
	if patchSize <= 0 {
		return 0, false
	}

	newSize := patchSize * expectedPerDim
	if newSize <= 0 || newSize == current {
		return 0, false
	}

	return newSize, true
}

func parseShapeFromError(msg, prefix string) ([]int, bool) {
	start := strings.Index(msg, prefix)
	if start == -1 {
		return nil, false
	}
	start += len(prefix)
	end := strings.Index(msg[start:], "}")
	if end == -1 {
		return nil, false
	}
	segment := msg[start : start+end]
	parts := strings.Split(segment, ",")
	dims := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.Atoi(part)
		if err != nil {
			return nil, false
		}
		dims = append(dims, v)
	}
	if len(dims) == 0 {
		return nil, false
	}
	return dims, true
}
