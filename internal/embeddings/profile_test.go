package embed

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileIdentifiesWeightsAndPreprocessing(t *testing.T) {
	previous := activeProfile.Load()
	t.Cleanup(func() { activeProfile.Store(previous) })
	dir := t.TempDir()
	for _, name := range []string{"vision_model_fp16.onnx", "text_model_fp16.onnx", "tokenizer.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := loadProfile(dir, 224); err != nil {
		t.Fatal(err)
	}
	first := ActiveProfile()
	if first.ID == "" || first.Dimensions != 768 || first.ImageSize != 224 {
		t.Fatalf("invalid profile: %+v", first)
	}
	if err := loadProfile(dir, 224); err != nil {
		t.Fatal(err)
	}
	if ActiveProfile() != first {
		t.Fatal("profile is not stable")
	}
	if err := loadProfile(dir, 256); err != nil {
		t.Fatal(err)
	}
	if ActiveProfile().ID == first.ID {
		t.Fatal("input dimensions not fingerprinted")
	}
	if err := os.WriteFile(filepath.Join(dir, "vision_model_fp16.onnx"), []byte("different model"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadProfile(dir, 224); err != nil {
		t.Fatal(err)
	}
	if ActiveProfile().ID == first.ID {
		t.Fatal("model weights not fingerprinted")
	}
}
