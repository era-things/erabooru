//go:build embeddings

package embed

import (
	"era/booru/internal/processing"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	tokenizer "github.com/sugarme/tokenizer"
	pretrained "github.com/sugarme/tokenizer/pretrained"
	ort "github.com/yalue/onnxruntime_go"
)

var (
	once             sync.Once
	loadErr          error
	dynSess          *ort.DynamicAdvancedSession
	outputName       string
	inputSpatialSize atomic.Int64

	textSess           *ort.DynamicAdvancedSession
	textOutputName     string
	textInputNames     []string
	textSequenceLength int
	textTokenizer      *tokenizer.Tokenizer
)

func init() {
	inputSpatialSize.Store(224)
}

func Available() bool { return true }

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

		visionPath := filepath.Join(dir, "vision_model_fp16.onnx")
		inputs, outputs, err := ort.GetInputOutputInfo(visionPath)
		if err != nil {
			loadErr = fmt.Errorf("failed to inspect ONNX model outputs: %w", err)
			return
		}
		if len(outputs) == 0 {
			loadErr = fmt.Errorf("vision model exposes no outputs")
			return
		}

		if size := resolveSpatialSize(inputs); size > 0 {
			setInputSpatialSize(size)
		}

		availableNames := make([]string, 0, len(outputs))
		for _, out := range outputs {
			availableNames = append(availableNames, out.Name)
		}

		outputName = chooseFloatOutput(outputs,
			"pooler_output",
			"image_embeds",
			"image_features",
			"img_embeds",
			"embeddings",
		)
		if outputName == "" {
			// Fall back to the first float tensor output if the preferred
			// names are absent so we can support future exports without
			// guessing the name.
			outputName = chooseFloatOutput(outputs, availableNames...)
		}

		if outputName == "" {
			loadErr = fmt.Errorf("vision model exposes no float tensor outputs (available: %s)", strings.Join(availableNames, ", "))
			return
		}

		sessionOpts, releaseSessionOpts := sessionOptionsWithCUDA()
		defer releaseSessionOpts()

		dynSess, loadErr = ort.NewDynamicAdvancedSession(
			visionPath,
			[]string{"pixel_values"}, // input
			[]string{outputName},     // output
			sessionOpts,
		)

		if loadErr != nil {
			return
		}

		tk, err := pretrained.FromFile(filepath.Join(dir, "tokenizer.json"))
		if err != nil {
			loadErr = err
			return
		}
		textTokenizer = tk

		textPath := filepath.Join(dir, "text_model_fp16.onnx")
		textInputs, textOutputs, err := ort.GetInputOutputInfo(textPath)
		if err != nil {
			loadErr = fmt.Errorf("failed to inspect text model outputs: %w", err)
			return
		}
		if len(textOutputs) == 0 {
			loadErr = fmt.Errorf("text model exposes no outputs")
			return
		}

		textInputNames = make([]string, len(textInputs))
		seqLen := 0
		for i, in := range textInputs {
			textInputNames[i] = in.Name
			if len(in.Dimensions) > 1 {
				if v := int(in.Dimensions[len(in.Dimensions)-1]); v > seqLen {
					seqLen = v
				}
			}
		}
		if seqLen <= 0 {
			seqLen = 64
		}
		textSequenceLength = seqLen

		availableTextOutputs := make([]string, 0, len(textOutputs))
		for _, out := range textOutputs {
			availableTextOutputs = append(availableTextOutputs, out.Name)
		}

		textOutputName = chooseFloatOutput(textOutputs,
			"text_embeds", "text_features", "txt_embeds", "embeddings", "pooler_output",
		)
		if textOutputName == "" {
			textOutputName = chooseFloatOutput(textOutputs, availableTextOutputs...)
		}
		if textOutputName == "" {
			loadErr = fmt.Errorf("text model exposes no float tensor outputs (available: %s)", strings.Join(availableTextOutputs, ", "))
			return
		}

		textSess, loadErr = ort.NewDynamicAdvancedSession(
			textPath,
			textInputNames,
			[]string{textOutputName},
			sessionOpts,
		)
		if loadErr == nil {
			loadErr = loadProfile(dir, InputSpatialSize())
		}
	})
	return loadErr
}

func chooseFloatOutput(outputs []ort.InputOutputInfo, names ...string) string {
	for _, name := range names {
		for _, output := range outputs {
			if output.Name == name && output.OrtValueType == ort.ONNXTypeTensor && output.DataType == ort.TensorElementDataTypeFloat {
				return output.Name
			}
		}
	}
	return ""
}

func sessionOptionsWithCUDA() (*ort.SessionOptions, func()) {
	options, err := ort.NewSessionOptions()
	if err != nil {
		log.Printf("embeddings: CUDA execution provider unavailable (creating session options failed): %v; using CPU execution provider", err)
		return nil, func() {}
	}

	cleanup := func() {
		if destroyErr := options.Destroy(); destroyErr != nil {
			log.Printf("embeddings: warning: failed to release session options: %v", destroyErr)
		}
	}
	threads := processing.ResourceThreads("ONNX_INTRA_THREADS", processing.DefaultInferenceThreads())
	log.Printf("embeddings: ONNX intra-op threads=%d", threads)
	if err := options.SetIntraOpNumThreads(threads); err != nil {
		log.Printf("embeddings: unable to set intra-op threads: %v", err)
	}
	if err := options.SetInterOpNumThreads(1); err != nil {
		log.Printf("embeddings: unable to set inter-op threads: %v", err)
	}
	if err := options.SetGraphOptimizationLevel(ort.GraphOptimizationLevelEnableAll); err != nil {
		log.Printf("embeddings: unable to enable graph optimizations: %v", err)
	}

	cudaOpts, err := ort.NewCUDAProviderOptions()
	if err != nil {
		log.Printf("embeddings: CUDA execution provider unavailable (creating provider options failed): %v; using CPU execution provider", err)
		return options, cleanup
	}

	if err := options.AppendExecutionProviderCUDA(cudaOpts); err != nil {
		log.Printf("embeddings: CUDA execution provider unavailable (append failed): %v; using CPU execution provider", err)
		if destroyErr := cudaOpts.Destroy(); destroyErr != nil {
			log.Printf("embeddings: warning: failed to release CUDA provider options: %v", destroyErr)
		}
		return options, cleanup
	}

	if err := cudaOpts.Destroy(); err != nil {
		log.Printf("embeddings: warning: failed to release CUDA provider options: %v", err)
	}

	log.Printf("embeddings: using CUDA execution provider for ONNX Runtime sessions")
	return options, cleanup
}

func Session() *ort.DynamicAdvancedSession { return dynSess }

func OutputName() string { return outputName }

func InputSpatialSize() int { return int(inputSpatialSize.Load()) }

func setInputSpatialSize(size int) {
	if size > 0 {
		inputSpatialSize.Store(int64(size))
	}
}

func resolveSpatialSize(inputs []ort.InputOutputInfo) int {
	var fallback int
	for _, in := range inputs {
		size := spatialSize(in)
		if size == 0 {
			continue
		}
		if in.Name == "pixel_values" {
			return size
		}
		if fallback == 0 {
			fallback = size
		}
	}
	return fallback
}

func spatialSize(in ort.InputOutputInfo) int {
	if in.OrtValueType != ort.ONNXTypeTensor {
		return 0
	}
	if in.DataType != ort.TensorElementDataTypeFloat {
		return 0
	}
	if len(in.Dimensions) < 4 {
		return 0
	}
	h := in.Dimensions[len(in.Dimensions)-2]
	w := in.Dimensions[len(in.Dimensions)-1]
	if h <= 0 || w <= 0 || h != w {
		return 0
	}
	return int(h)
}
