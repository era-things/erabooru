package embed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
)

// Profile identifies the model files and preprocessing contract used for imports.
type Profile struct {
	ID                 string `json:"id,omitempty"`
	Dimensions         int    `json:"dimensions"`
	ImageSize          int    `json:"image_size"`
	VisionSHA256       string `json:"vision_sha256"`
	TextSHA256         string `json:"text_sha256"`
	TokenizerSHA256    string `json:"tokenizer_sha256"`
	ImagePreprocessing string `json:"image_preprocessing"`
	VideoAggregation   string `json:"video_aggregation"`
}

var activeProfile atomic.Pointer[Profile]

func ActiveProfile() Profile {
	if p := activeProfile.Load(); p != nil {
		return *p
	}
	return Profile{}
}

func loadProfile(dir string, size int) error {
	p := Profile{Dimensions: 768, ImageSize: size, ImagePreprocessing: "preview512-center-crop-srgb-black-alpha-rgb-minus1-plus1-v1", VideoAggregation: "uniform-samples-5-8-12-16-letterbox-black-mean-l2-v1"}
	for _, file := range []struct {
		name        string
		destination *string
	}{
		{"vision_model_fp16.onnx", &p.VisionSHA256}, {"text_model_fp16.onnx", &p.TextSHA256}, {"tokenizer.json", &p.TokenizerSHA256},
	} {
		f, err := os.Open(filepath.Join(dir, file.name))
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		*file.destination = hex.EncodeToString(h.Sum(nil))
	}
	p = p.ForImageSize(size)
	activeProfile.Store(&p)
	return nil
}

// ForImageSize fingerprints the preprocessing actually used for one inference,
// including the fallback size discovered from an ONNX shape error.
func (p Profile) ForImageSize(size int) Profile {
	if p.VisionSHA256 == "" || (p.ImageSize == size && p.ID != "") {
		return p
	}
	p.ImageSize = size
	p.ID = ""
	data, _ := json.Marshal(p) // Profile contains only strings and integers.
	hash := sha256.Sum256(data)
	p.ID = fmt.Sprintf("siglip2-%x", hash)
	return p
}
