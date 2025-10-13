//go:build embeddings

package embed

import "testing"

func TestNormalizeRGB24(t *testing.T) {
	const S = 2
	raw := []byte{
		0, 127, 255,
		64, 128, 192,
		255, 255, 255,
		10, 20, 30,
	}

	tensor, err := normalizeRGB24(raw, S)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLen := 3 * S * S
	if len(tensor) != expectedLen {
		t.Fatalf("unexpected tensor length %d", len(tensor))
	}

	norm := func(v byte) float32 {
		return (float32(v)/255 - .5) / .5
	}

	expectedR := []float32{norm(0), norm(64), norm(255), norm(10)}
	expectedG := []float32{norm(127), norm(128), norm(255), norm(20)}
	expectedB := []float32{norm(255), norm(192), norm(255), norm(30)}

	for i, want := range expectedR {
		if got := tensor[i]; !almostEqual(got, want) {
			t.Fatalf("unexpected R channel value at %d: got %f, want %f", i, got, want)
		}
	}
	for i, want := range expectedG {
		idx := S*S + i
		if got := tensor[idx]; !almostEqual(got, want) {
			t.Fatalf("unexpected G channel value at %d: got %f, want %f", i, got, want)
		}
	}
	for i, want := range expectedB {
		idx := 2*S*S + i
		if got := tensor[idx]; !almostEqual(got, want) {
			t.Fatalf("unexpected B channel value at %d: got %f, want %f", i, got, want)
		}
	}
}

func TestNormalizeRGB24InvalidLength(t *testing.T) {
	const S = 2
	_, err := normalizeRGB24(make([]byte, 3*S*S-1), S)
	if err == nil {
		t.Fatalf("expected error for short buffer")
	}
}

func almostEqual(a, b float32) bool {
	if a == b {
		return true
	}
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 1e-4
}
