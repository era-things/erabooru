//go:build embeddings

package embed

import (
	"fmt"
	"image"
	"math"

	"github.com/disintegration/imaging"
	ort "github.com/yalue/onnxruntime_go"
)

// VisionEmbedding converts an image to an L2-normalised embedding vector.
func VisionEmbedding(img image.Image) ([]float32, error) {
	S := InputSpatialSize()
	if S <= 0 {
		return nil, fmt.Errorf("invalid vision input size %d", S)
	}
	// 1) centre-crop & resize
	img = imaging.Fill(img, S, S, imaging.Center, imaging.Lanczos)

	// 2) HWC uint8 → NCHW float32 in [-1, 1]
	pix := make([]float32, 3*S*S)
	for y := 0; y < S; y++ {
		for x := 0; x < S; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			i := y*S + x
			pix[i] = (float32(r>>8)/255 - .5) / .5
			pix[S*S+i] = (float32(g>>8)/255 - .5) / .5
			pix[2*S*S+i] = (float32(b>>8)/255 - .5) / .5
		}
	}

	// 3) build input tensor
	in, err := ort.NewTensor[float32](ort.NewShape(1, 3, int64(S), int64(S)), pix)
	if err != nil {
		return nil, err
	}
	defer in.Destroy()

	// 4) run the model and let ORT allocate the output so we can support both
	// pooled embeddings (image_embeds) and full token grids (last_hidden_state).
	outputs := []ort.Value{nil}
	if err := Session().Run(
		[]ort.Value{in}, // inputs
		outputs,         // outputs (allocated by ORT)
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
	if shape[0] != 1 {
		return nil, fmt.Errorf("unexpected batch dimension %d", shape[0])
	}

	var vec []float32
	switch len(shape) {
	case 2:
		// [batch, dim]
		dim := int(shape[1])
		if len(data) != dim {
			return nil, fmt.Errorf("embedding data length mismatch: got %d, expected %d", len(data), dim)
		}
		vec = make([]float32, dim)
		copy(vec, data)
	case 3:
		// [batch, tokens, dim]; take the first token which corresponds to the
		// pooled representation for SigLIP vision towers.
		tokens := int(shape[1])
		dim := int(shape[2])
		expected := tokens * dim
		if len(data) != expected {
			return nil, fmt.Errorf("embedding data length mismatch: got %d, expected %d", len(data), expected)
		}
		if tokens == 0 {
			return nil, fmt.Errorf("embedding has zero tokens")
		}
		vec = make([]float32, dim)
		copy(vec, data[:dim])
	default:
		return nil, fmt.Errorf("unsupported embedding rank %d", len(shape))
	}

	l2(vec)
	return vec, nil
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
