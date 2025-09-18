//go:build embeddings

package embed

import (
	"image"
	"math"

	"github.com/disintegration/imaging"
	ort "github.com/yalue/onnxruntime_go"
)

// VisionEmbedding converts an image to a 768-D L2-normalised vector.
func VisionEmbedding(img image.Image) ([]float32, error) {
	const S = 384 // SigLIP-2 Base resolution
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

	// 3) build ORT tensors
	in, err := ort.NewTensor[float32](ort.NewShape(1, 3, S, S), pix)
	if err != nil {
		return nil, err
	}
	defer in.Destroy()

	out, err := ort.NewEmptyTensor[float32](ort.NewShape(1, 768))
	if err != nil {
		return nil, err
	}
	defer out.Destroy()

	// 4) run the model
	if err := Session().Run(
		[]ort.Value{in},  // inputs
		[]ort.Value{out}, // outputs (filled in place)
	); err != nil {
		return nil, err
	}

	// 5) copy, normalise, return
	vec := make([]float32, 768)
	copy(vec, out.GetData())
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
