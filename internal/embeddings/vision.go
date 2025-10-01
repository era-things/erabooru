//go:build embeddings

package embed

import (
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/cshum/vipsgen/vips"
	ort "github.com/yalue/onnxruntime_go"
)

var vipsOnce sync.Once

func ensureVips() {
	vipsOnce.Do(func() {
		vips.Startup(&vips.Config{ConcurrencyLevel: runtime.NumCPU()})
	})
}

// VisionEmbedding converts an image buffer to an L2-normalised embedding vector.
func VisionEmbedding(buf []byte) ([]float32, error) {
	if len(buf) == 0 {
		return nil, fmt.Errorf("vision embedding: empty image buffer")
	}

	ensureVips()

	const maxAttempts = 3

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		S := InputSpatialSize()
		vec, err := visionEmbeddingWithSize(buf, S)
		if err == nil {
			return vec, nil
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

func visionEmbeddingWithSize(buf []byte, S int) ([]float32, error) {
	if S <= 0 {
		return nil, fmt.Errorf("invalid vision input size %d", S)
	}

	// 1) Do NOT set loader autorotate — PNG/WebP PNG path will reject it.
	loadOptions := vips.DefaultLoadOptions()
	// loadOptions.Autorotate = true // <-- remove

	// (Optional) If you want autorotation for JPEGs only:
	// ct := http.DetectContentType(buf)
	// if strings.Contains(ct, "jpeg") || strings.Contains(ct, "jpg") {
	//     loadOptions.Autorotate = true
	// }

	thumbOptions := &vips.ThumbnailBufferOptions{
		OptionString: loadOptions.OptionString(),
		Height:       S,
		Size:         vips.SizeBoth,
		Crop:         vips.InterestingCentre,
		// FailOnError will turn unknown loader options into hard errors.
		// Since we removed Autorotate above, we can keep this strict default.
		FailOn: vips.FailOnError,
	}

	img, err := vips.NewThumbnailBuffer(buf, S, thumbOptions)
	if err != nil {
		return nil, fmt.Errorf("vips thumbnail: %w", err)
	}
	defer func() {
		if img != nil {
			img.Close()
		}
	}()

	// 2) Optional post-load autorotate (safe for all formats; no EXIF -> no-op).
	// If your binding exposes Autorot:
	if err := img.Autorot(); err != nil {
		// ignore "not supported" style errors; treat only unexpected ones as fatal
		// (you can log at debug if you like)
	}

	if err := img.Colourspace(vips.InterpretationSrgb, nil); err != nil {
		return nil, fmt.Errorf("vips colourspace: %w", err)
	}

	if img.HasAlpha() {
		if err := img.Flatten(&vips.FlattenOptions{Background: []float64{0, 0, 0}}); err != nil {
			return nil, fmt.Errorf("vips flatten: %w", err)
		}
	}

	bands := img.Bands()
	switch {
	case bands > 3:
		if err := img.ExtractBand(0, &vips.ExtractBandOptions{N: 3}); err != nil {
			return nil, fmt.Errorf("vips extract band: %w", err)
		}
		bands = img.Bands()
	case bands < 3:
		// Expand to RGB by joining bands with themselves.
		bandImages := make([]*vips.Image, 0, 3)
		for bands < 3 {
			dup, dupErr := img.Copy(nil)
			if dupErr != nil {
				for _, im := range bandImages {
					im.Close()
				}
				return nil, fmt.Errorf("vips duplicate band: %w", dupErr)
			}
			bandImages = append(bandImages, dup)
			bands++
		}
		joined, joinErr := vips.NewBandjoin(append([]*vips.Image{img}, bandImages...))
		for _, im := range bandImages {
			im.Close()
		}
		if joinErr != nil {
			return nil, fmt.Errorf("vips bandjoin: %w", joinErr)
		}
		old := img
		img = joined
		old.Close()
		bands = img.Bands()
	}

	if bands != 3 {
		return nil, fmt.Errorf("unexpected band count %d", bands)
	}

	if img.BandFormat() != vips.BandFormatUchar {
		if err := img.Cast(vips.BandFormatUchar, nil); err != nil {
			return nil, fmt.Errorf("vips cast: %w", err)
		}
	}

	width := img.Width()
	height := img.Height()
	if width != S || height != S {
		return nil, fmt.Errorf("unexpected thumbnail size %dx%d (expected %d)", width, height, S)
	}

	raw, err := img.RawsaveBuffer(nil)
	if err != nil {
		return nil, fmt.Errorf("vips rawsave: %w", err)
	}
	if len(raw) != 3*S*S {
		return nil, fmt.Errorf("unexpected raw buffer length %d (expected %d)", len(raw), 3*S*S)
	}

	pix := make([]float32, 3*S*S)
	for y := 0; y < S; y++ {
		rowOffset := y * S
		for x := 0; x < S; x++ {
			idx := rowOffset + x
			base := idx * 3
			pix[idx] = (float32(raw[base])/255 - .5) / .5
			pix[S*S+idx] = (float32(raw[base+1])/255 - .5) / .5
			pix[2*S*S+idx] = (float32(raw[base+2])/255 - .5) / .5
		}
	}

	in, err := ort.NewTensor[float32](ort.NewShape(1, 3, int64(S), int64(S)), pix)
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
	if shape[0] != 1 {
		return nil, fmt.Errorf("unexpected batch dimension %d", shape[0])
	}

	var vec []float32
	switch len(shape) {
	case 2:
		dim := int(shape[1])
		if len(data) != dim {
			return nil, fmt.Errorf("embedding data length mismatch: got %d, expected %d", len(data), dim)
		}
		vec = make([]float32, dim)
		copy(vec, data)
	default:
		return nil, fmt.Errorf("unsupported embedding rank %d", len(shape))
	}

	l2(vec)

	return vec, nil
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
