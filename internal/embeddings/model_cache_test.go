//go:build embeddings

package embed

import "testing"

func TestParseModelFiles(t *testing.T) {
	files, err := parseModelFiles("a|remote/a|abc,b||,c|http://example.com/file|DEF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	if files[0].LocalPath != "a" || files[0].RemotePath != "remote/a" || files[0].SHA256 != "abc" {
		t.Fatalf("unexpected file[0]: %+v", files[0])
	}
	if files[1].LocalPath != "b" || files[1].RemotePath != "b" || files[1].SHA256 != "" {
		t.Fatalf("unexpected file[1]: %+v", files[1])
	}
	if files[2].LocalPath != "c" || files[2].RemotePath != "http://example.com/file" || files[2].SHA256 != "def" {
		t.Fatalf("unexpected file[2]: %+v", files[2])
	}
}

func TestBuildFileURL(t *testing.T) {
	opts := ModelOptions{Repository: defaultRepo, Revision: "main"}
	u := buildFileURL(opts, ModelFile{LocalPath: "vision.onnx", RemotePath: "onnx/vision.onnx"})
	expected := "https://huggingface.co/onnx-community/siglip2-base-patch16-224-ONNX/resolve/main/onnx/vision.onnx?download=1"
	if u != expected {
		t.Fatalf("unexpected hugging face url: %s", u)
	}

	opts = ModelOptions{Repository: "https://example.com/models", Revision: "v1"}
	u = buildFileURL(opts, ModelFile{LocalPath: "vision.onnx", RemotePath: "vision.onnx"})
	if u != "https://example.com/models/v1/vision.onnx" {
		t.Fatalf("unexpected absolute base url: %s", u)
	}
}

func TestDefaultModelOptionsFromEnv(t *testing.T) {
	t.Setenv("MODEL_CACHE_DIR", "")
	t.Setenv("MODEL_NAME", "")
	t.Setenv("MODEL_REPOSITORY", "")
	t.Setenv("MODEL_REVISION", "")
	t.Setenv("MODEL_FILES", "token|tokenizer.json|123")

	opts := DefaultModelOptionsFromEnv()
	if opts.CacheDir != defaultCacheDir {
		t.Fatalf("expected default cache dir, got %s", opts.CacheDir)
	}
	if opts.ModelName != defaultModelName {
		t.Fatalf("expected default model name, got %s", opts.ModelName)
	}
	if opts.Repository != defaultRepo {
		t.Fatalf("expected default repo, got %s", opts.Repository)
	}
	if opts.Revision != defaultRevision {
		t.Fatalf("expected default revision, got %s", opts.Revision)
	}
	if len(opts.Files) != 1 || opts.Files[0].LocalPath != "token" {
		t.Fatalf("expected custom files override, got %+v", opts.Files)
	}
	if opts.Client == nil {
		t.Fatalf("expected client to be initialised")
	}
}
