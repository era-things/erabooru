package embed

import (
	"path/filepath"
	"sync"

	ort "github.com/ivansuteja96/go-onnxruntime"
)

var (
	once    sync.Once
	model   *ort.Session
	loadErr error
)

// dir = absolute path to the model directory (passed from main)
func Load(dir string) error {
	once.Do(func() {
		modelPath := filepath.Join(dir, "vision_model_int8.onnx")
		model, loadErr = ort.NewSession(modelPath)
	})
	return loadErr
}

func Session() *ort.Session { return model }
