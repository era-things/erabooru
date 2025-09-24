//go:build embeddings

package embed

import (
	"fmt"
	"strings"

	ort "github.com/yalue/onnxruntime_go"
)

// TextEmbedding converts a text query into an L2-normalised embedding vector.
func TextEmbedding(text string) ([]float32, error) {
	if textSess == nil || textTokenizer == nil {
		return nil, fmt.Errorf("text embedding model not loaded")
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, fmt.Errorf("text query is empty")
	}

	encoding, err := textTokenizer.EncodeSingle(trimmed)
	if err != nil {
		return nil, fmt.Errorf("tokenize text: %w", err)
	}

	ids := encoding.Ids
	mask := encoding.AttentionMask
	if len(ids) == 0 {
		return nil, fmt.Errorf("tokenizer produced no tokens")
	}
	if len(mask) == 0 {
		mask = make([]int, len(ids))
		for i := range mask {
			mask[i] = 1
		}
	}

	targetLen := len(ids)
	if textSequenceLength > 0 {
		targetLen = textSequenceLength
	}

	ids64 := make([]int64, targetLen)
	mask64 := make([]int64, targetLen)
	for i := 0; i < targetLen; i++ {
		if i < len(ids) {
			ids64[i] = int64(ids[i])
			if i < len(mask) {
				mask64[i] = int64(mask[i])
			} else {
				mask64[i] = 1
			}
		} else {
			ids64[i] = 0
			mask64[i] = 0
		}
	}

	idsTensor, err := ort.NewTensor[int64](ort.NewShape(1, int64(targetLen)), ids64)
	if err != nil {
		return nil, err
	}
	defer idsTensor.Destroy()

	var attentionTensor *ort.Tensor[int64]
	var positionTensor *ort.Tensor[int64]
	var tokenTypeTensor *ort.Tensor[int64]

	inputs := make([]ort.Value, len(textInputNames))
	for i, name := range textInputNames {
		switch name {
		case "input_ids":
			inputs[i] = idsTensor
		case "attention_mask":
			if attentionTensor == nil {
				attentionTensor, err = ort.NewTensor[int64](ort.NewShape(1, int64(targetLen)), mask64)
				if err != nil {
					return nil, err
				}
				defer attentionTensor.Destroy()
			}
			inputs[i] = attentionTensor
		case "position_ids":
			if positionTensor == nil {
				positionData := make([]int64, targetLen)
				for j := range positionData {
					positionData[j] = int64(j)
				}
				positionTensor, err = ort.NewTensor[int64](ort.NewShape(1, int64(targetLen)), positionData)
				if err != nil {
					return nil, err
				}
				defer positionTensor.Destroy()
			}
			inputs[i] = positionTensor
		case "token_type_ids":
			if tokenTypeTensor == nil {
				tokenTypeData := make([]int64, targetLen)
				tokenTypeTensor, err = ort.NewTensor[int64](ort.NewShape(1, int64(targetLen)), tokenTypeData)
				if err != nil {
					return nil, err
				}
				defer tokenTypeTensor.Destroy()
			}
			inputs[i] = tokenTypeTensor
		default:
			return nil, fmt.Errorf("unsupported text model input %s", name)
		}
	}

	outputs := []ort.Value{nil}
	if err := textSess.Run(inputs, outputs); err != nil {
		return nil, fmt.Errorf("failed to run text model: %w", err)
	}

	out, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected text embedding tensor type %T", outputs[0])
	}
	defer out.Destroy()

	data := out.GetData()
	shape := out.GetShape()

	var vec []float32
	switch len(shape) {
	case 2:
		dim := int(shape[1])
		if len(data) != dim {
			return nil, fmt.Errorf("embedding data length mismatch: got %d, expected %d", len(data), dim)
		}
		vec = make([]float32, dim)
		copy(vec, data)
	case 3:
		tokens := int(shape[1])
		dim := int(shape[2])
		expected := tokens * dim
		if len(data) != expected {
			return nil, fmt.Errorf("embedding data length mismatch: got %d, expected %d", len(data), expected)
		}
		if tokens == 0 {
			return nil, fmt.Errorf("embedding has zero tokens")
		}
		vec = make([]float32, dim)
		copy(vec, data[:dim])
	default:
		return nil, fmt.Errorf("unsupported embedding rank %d", len(shape))
	}

	l2(vec)
	return vec, nil
}
