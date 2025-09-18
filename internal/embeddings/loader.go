//go:build embeddings

package embed

import (
	"os"
	"path/filepath"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var (
	once    sync.Once
	loadErr error
	dynSess *ort.DynamicAdvancedSession
)

// Load must be called once (e.g. from main). dir contains vision_model_fp16.onnx
func Load(dir string) error {
	once.Do(func() {
		// init the global ORT environment once
		if !ort.IsInitialized() {
			if err := ort.InitializeEnvironment(); err != nil {
				loadErr = err
				return
			}
			// optional: lower ORT logging noise
			_ = ort.SetEnvironmentLogLevel(ort.LoggingLevelError)
		}

		// read model bytes
		onx, err := os.ReadFile(filepath.Join(dir, "vision_model_fp16.onnx"))
		if err != nil {
			loadErr = err
			return
		}

		// create a dynamic session; change output name if your model differs
		dynSess, loadErr = ort.NewDynamicAdvancedSessionWithONNXData(
			onx,
			[]string{"pixel_values"},      // input
			[]string{"last_hidden_state"}, // output (sometimes "image_embeds")
			nil,                           // default SessionOptions
		)
	})
	return loadErr
}

func Session() *ort.DynamicAdvancedSession { return dynSess }
