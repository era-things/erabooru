package embed

import (
	"image"
	"math"

	"github.com/disintegration/imaging"
	ort "github.com/ivansuteja96/go-onnxruntime"
	"gorgonia.org/tensor"
)

func VisionEmbedding(img image.Image) ([]float32, error) {
	// 1. centre-crop & resize to 384×384 (SigLIP-2 default)
	cropped := imaging.Fill(img, 384, 384, imaging.Center, imaging.Lanczos)

	// 2. to NCHW float32 in range [-1, 1]
	w, h := 384, 384
	pix := make([]float32, 3*w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := cropped.At(x, y).RGBA()
			i := y*w + x
			pix[i] = (float32(r>>8)/255.0 - 0.5) / 0.5       // R
			pix[w*h+i] = (float32(g>>8)/255.0 - 0.5) / 0.5   // G
			pix[2*w*h+i] = (float32(b>>8)/255.0 - 0.5) / 0.5 // B
		}
	}

	t := tensor.New(tensor.WithShape(1, 3, h, w), tensor.WithBacking(pix))
	out, err := Session().Run([]ort.NamedTensor{{Name: "pixel_values", T: t}})
	if err != nil {
		return nil, err
	}
	vec := out[0].Data().([]float32)
	return l2norm(vec), nil
}

func l2norm(v []float32) []float32 {
	var sum float32
	for _, x := range v {
		sum += x * x
	}
	scale := 1 / float32(math.Sqrt(float64(sum)))
	for i := range v {
		v[i] *= scale
	}
	return v
}
