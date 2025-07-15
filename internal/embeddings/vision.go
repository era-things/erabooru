package embed

import (
	"image"
	"math"

	"github.com/disintegration/imaging"
	"gorgonia.org/tensor"
)

// VisionEmbedding converts an RGB image into a 768-D L2-normalised vector.
func VisionEmbedding(img image.Image) ([]float32, error) {
	const size = 384 // SigLIP-2 base resolution

	// 1. centre-crop & resize
	img = imaging.Fill(img, size, size, imaging.Center, imaging.Lanczos)

	// 2. HWC uint8 → NCHW float32 in [-1,1]
	pix := make([]float32, 3*size*size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			i := y*size + x
			pix[i] = (float32(r>>8)/255 - 0.5) / 0.5             // R
			pix[size*size+i] = (float32(g>>8)/255 - 0.5) / 0.5   // G
			pix[2*size*size+i] = (float32(b>>8)/255 - 0.5) / 0.5 // B
		}
	}
	input := tensor.New(tensor.WithShape(1, 3, size, size),
		tensor.WithBacking(pix))

	// 3. run inference
	outputs, err := Session().Run(map[string]*tensor.Dense{
		"pixel_values": input,
	})
	if err != nil {
		return nil, err
	}

	vec := outputs["last_hidden_state"].Data().([]float32) // 1×768
	return l2norm(vec), nil
}

func l2norm(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	scale := float32(1.0 / math.Sqrt(sum))
	for i := range v {
		v[i] *= scale
	}
	return v
}
