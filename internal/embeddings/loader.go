//go:build embeddings

package embed

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

		_, outputs, err := ort.GetInputOutputInfoWithONNXData(onx)
		if err != nil {
			loadErr = fmt.Errorf("failed to inspect ONNX model outputs: %w", err)
			return
		}
		if len(outputs) == 0 {
			loadErr = fmt.Errorf("vision model exposes no outputs")
			return
		}

		availableNames := make([]string, 0, len(outputs))
		for _, out := range outputs {
			availableNames = append(availableNames, out.Name)
		}

		chooseOutput := func(names ...string) string {
			for _, name := range names {
				for _, out := range outputs {
					if out.Name != name {
						continue
					}
					if out.OrtValueType != ort.ONNXTypeTensor {
						continue
					}
					if out.DataType != ort.TensorElementDataTypeFloat {
						continue
					}
					return out.Name
				}
			}
			return ""
		}

		outputName = chooseOutput("image_embeds", "last_hidden_state")
		if outputName == "" {
			// Fall back to the first float tensor output if the preferred
			// names are absent so we can support future exports without
			// guessing the name.
			outputName = chooseOutput(availableNames...)
		}

		if outputName == "" {
			loadErr = fmt.Errorf("vision model exposes no float tensor outputs (available: %s)", strings.Join(availableNames, ", "))
			return
		}

		dynSess, loadErr = ort.NewDynamicAdvancedSessionWithONNXData(
			onx,
			[]string{"pixel_values"}, // input
			[]string{outputName},     // output
			nil,                      // default SessionOptions
		)
	})
	return loadErr
}

func Session() *ort.DynamicAdvancedSession { return dynSess }

func OutputName() string { return outputName }
