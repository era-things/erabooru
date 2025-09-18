//go:build embeddings

package embed

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var (
	once       sync.Once
	loadErr    error
	dynSess    *ort.DynamicAdvancedSession
	outputName string
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

		// create a dynamic session; attempt multiple output names because
		// different SigLIP exports expose either "image_embeds" or
		// "last_hidden_state".
		for _, candidate := range []string{"image_embeds", "last_hidden_state"} {
			var sessErr error
			dynSess, sessErr = ort.NewDynamicAdvancedSessionWithONNXData(
				onx,
				[]string{"pixel_values"}, // input
				[]string{candidate},      // output
				nil,                      // default SessionOptions
			)
			if sessErr == nil {
				outputName = candidate
				return
			}
			loadErr = sessErr
		}

		if dynSess == nil && loadErr == nil {
			loadErr = fmt.Errorf("failed to create ONNX session: no compatible output name found")
		}
	})
	return loadErr
}

func Session() *ort.DynamicAdvancedSession { return dynSess }

func OutputName() string { return outputName }
