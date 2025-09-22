//go:build embeddings

package embed

import (
	"fmt"
	"strings"

	"github.com/sugarme/tokenizer"
	ort "github.com/yalue/onnxruntime_go"
)

// TextEmbedding converts a piece of text into an embedding vector using the
// loaded text model. The resulting vector is L2-normalised to match the vision
// embeddings stored in the database so that cosine similarity can be used
// directly.
func TextEmbedding(text string) ([]float32, error) {
	if textSession() == nil || tokenizerInstance() == nil {
		return nil, fmt.Errorf("text embedding model not initialised")
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, fmt.Errorf("empty text input")
	}

	input := tokenizer.NewInputSequence(trimmed)
	enc, err := tokenizerInstance().Encode(tokenizer.NewSingleEncodeInput(input), true)
	if err != nil {
		return nil, fmt.Errorf("tokenize text: %w", err)
	}
	if len(enc.Ids) == 0 {
		return nil, fmt.Errorf("tokenizer returned no tokens")
	}
	if len(enc.AttentionMask) != len(enc.Ids) {
		return nil, fmt.Errorf("attention mask length mismatch: %d vs %d", len(enc.AttentionMask), len(enc.Ids))
	}

	seqLen := len(enc.Ids)
	inputsInfo := textModelInputs()
	if len(inputsInfo) == 0 {
		return nil, fmt.Errorf("text model exposes no inputs")
	}

	inputs := make([]ort.Value, len(inputsInfo))
	defer func() {
		for _, v := range inputs {
			if v != nil {
				v.Destroy()
			}
		}
	}()

	for i, info := range inputsInfo {
		name := strings.ToLower(info.Name)
		shape := ort.NewShape(1, int64(seqLen))
		switch {
		case strings.Contains(name, "input") && strings.Contains(name, "id"):
			data := make([]int64, seqLen)
			for idx, id := range enc.Ids {
				data[idx] = int64(id)
			}
			tensor, err := ort.NewTensor[int64](shape, data)
			if err != nil {
				return nil, fmt.Errorf("create input_ids tensor: %w", err)
			}
			inputs[i] = tensor
		case strings.Contains(name, "attention"):
			data := make([]int64, seqLen)
			for idx, v := range enc.AttentionMask {
				data[idx] = int64(v)
			}
			tensor, err := ort.NewTensor[int64](shape, data)
			if err != nil {
				return nil, fmt.Errorf("create attention_mask tensor: %w", err)
			}
			inputs[i] = tensor
		case strings.Contains(name, "position"):
			data := make([]int64, seqLen)
			for idx := range data {
				data[idx] = int64(idx)
			}
			tensor, err := ort.NewTensor[int64](shape, data)
			if err != nil {
				return nil, fmt.Errorf("create position_ids tensor: %w", err)
			}
			inputs[i] = tensor
		case strings.Contains(name, "token_type"):
			tensor, err := ort.NewTensor[int64](shape, make([]int64, seqLen))
			if err != nil {
				return nil, fmt.Errorf("create token_type_ids tensor: %w", err)
			}
			inputs[i] = tensor
		default:
			return nil, fmt.Errorf("unsupported text model input %q", info.Name)
		}
	}

	outputs := []ort.Value{nil}
	if err := textSession().Run(inputs, outputs); err != nil {
		return nil, fmt.Errorf("run text model: %w", err)
	}

	out, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected text model output type %T", outputs[0])
	}
	defer out.Destroy()

	data := out.GetData()
	shape := out.GetShape()
	if len(shape) < 2 {
		return nil, fmt.Errorf("unexpected text embedding rank %d", len(shape))
	}

	var vec []float32
	switch len(shape) {
	case 2:
		dim := int(shape[1])
		if len(data) < dim {
			return nil, fmt.Errorf("embedding output shorter than expected: %d < %d", len(data), dim)
		}
		vec = make([]float32, dim)
		copy(vec, data[:dim])
	case 3:
		tokens := int(shape[1])
		dim := int(shape[2])
		expected := tokens * dim
		if len(data) < expected {
			return nil, fmt.Errorf("embedding output shorter than expected: %d < %d", len(data), expected)
		}
		if tokens == 0 {
			return nil, fmt.Errorf("text embedding returned zero tokens")
		}

		tokenIndex := tokens - 1
		if strings.EqualFold(textModelOutputName(), "last_hidden_state") {
			tokenIndex = 0
		}
		if tokenIndex < 0 || tokenIndex >= tokens {
			return nil, fmt.Errorf("selected token %d out of range (tokens=%d)", tokenIndex, tokens)
		}

		start := tokenIndex * dim
		end := start + dim
		if start < 0 || end > len(data) {
			return nil, fmt.Errorf("text embedding token slice out of range: %d-%d (len=%d)", start, end, len(data))
		}
		vec = make([]float32, dim)
		copy(vec, data[start:end])
	default:
		return nil, fmt.Errorf("unsupported text embedding rank %d", len(shape))
	}

	l2(vec)
	return vec, nil
}
