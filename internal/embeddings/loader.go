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

// Load must be called exactly once (e.g. from main).
// dir → absolute path that contains vision_model_int8.onnx
func Load(dir string) error {
	once.Do(func() {
		/* ①  initialise the global ORT runtime if it isn’t already */
		if !ort.IsInitialized() {
			if err := ort.InitializeEnvironment(); err != nil {
				loadErr = err
				return
			}
		}

		/* ②  read the model bytes */
		onx, err := os.ReadFile(filepath.Join(dir, "vision_model_int8.onnx"))
		if err != nil {
			loadErr = err
			return
		}

		/* ③  build a DynamicAdvancedSession (no env argument) */
		dynSess, loadErr = ort.NewDynamicAdvancedSessionWithONNXData(
			onx,
			[]string{"pixel_values"},      // inputs
			[]string{"last_hidden_state"}, // outputs
			nil,                           // default SessionOptions
		)
	})
	return loadErr
}

// Session gives access to the singleton session after Load().
func Session() *ort.DynamicAdvancedSession { return dynSess }
