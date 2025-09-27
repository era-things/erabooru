//go:build embeddings

package embed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ModelFile struct {
	LocalPath  string
	RemotePath string
	SHA256     string
}

type ModelOptions struct {
	CacheDir   string
	ModelName  string
	Repository string
	Revision   string
	Files      []ModelFile
	Client     *http.Client
}

const (
	defaultCacheDir  = "/cache/models"
	defaultModelName = "Siglip2_FP16"
	defaultRepo      = "onnx-community/siglip2-base-patch16-224-ONNX"
	defaultRevision  = "main"
)

var defaultModelFiles = []ModelFile{
	{
		LocalPath:  "vision_model_fp16.onnx",
		RemotePath: "onnx/vision_model_fp16.onnx",
		SHA256:     "7596d306407eca5d4d3c45125919f7af534aa6118535422f5dbd1174c4fab55b",
	},
	{
		LocalPath:  "text_model_fp16.onnx",
		RemotePath: "onnx/text_model_fp16.onnx",
		SHA256:     "cb227819fd0ae9e3fdd34c9961dfc5a059449b291483da0e8a387ef2d4b51f1d",
	},
	{
		LocalPath:  "tokenizer.json",
		RemotePath: "tokenizer.json",
		SHA256:     "cb9140fae3ac5122c972d37adf83e1248471a38147ad76f8215c8872c6fd8322",
	},
}

func DefaultModelOptionsFromEnv() ModelOptions {
	opts := ModelOptions{
		CacheDir:   getEnvOrDefault("MODEL_CACHE_DIR", defaultCacheDir),
		ModelName:  getEnvOrDefault("MODEL_NAME", defaultModelName),
		Repository: getEnvOrDefault("MODEL_REPOSITORY", defaultRepo),
		Revision:   getEnvOrDefault("MODEL_REVISION", defaultRevision),
		Files:      defaultModelFiles,
	}

	if raw := strings.TrimSpace(os.Getenv("MODEL_FILES")); raw != "" {
		files, err := parseModelFiles(raw)
		if err != nil {
			log.Printf("embeddings: %v", err)
		} else if len(files) > 0 {
			opts.Files = files
		}
	}

	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 10 * time.Minute}
	}

	return opts
}

func EnsureModel(ctx context.Context, opts ModelOptions) (string, error) {
	if opts.CacheDir == "" {
		return "", errors.New("model cache directory is empty")
	}
	if opts.ModelName == "" {
		return "", errors.New("model name is empty")
	}
	if len(opts.Files) == 0 {
		return "", errors.New("no model files configured")
	}
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 10 * time.Minute}
	}

	targetDir := filepath.Join(opts.CacheDir, opts.ModelName)
	for _, file := range opts.Files {
		if err := ensureFile(ctx, opts, targetDir, file); err != nil {
			return "", err
		}
	}
	return targetDir, nil
}

func ensureFile(ctx context.Context, opts ModelOptions, base string, file ModelFile) error {
	if file.LocalPath == "" {
		return fmt.Errorf("model file has empty local path")
	}
	localPath := filepath.Join(base, filepath.FromSlash(file.LocalPath))
	if ok, err := validateExistingFile(localPath, file.SHA256); err != nil {
		return err
	} else if ok {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create model directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(localPath), ".download-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, buildFileURL(opts, file), nil)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", file.RemotePath, err)
	}

	resp, err := opts.Client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", req.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected status %s", req.URL, resp.Status)
	}

	hasher := sha256.New()
	writer := io.MultiWriter(tmp, hasher)
	if _, err := io.Copy(writer, resp.Body); err != nil {
		return fmt.Errorf("copy %s: %w", req.URL, err)
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if file.SHA256 != "" {
		sum := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(sum, file.SHA256) {
			return fmt.Errorf("checksum mismatch for %s: expected %s got %s", file.LocalPath, file.SHA256, sum)
		}
	}

	if err := os.Rename(tmp.Name(), localPath); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func validateExistingFile(path, expectedHash string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("%s is not a regular file", path)
	}
	if expectedHash == "" {
		if info.Size() == 0 {
			return false, nil
		}
		return true, nil
	}
	sum, err := fileSHA256(path)
	if err != nil {
		return false, err
	}
	if strings.EqualFold(sum, expectedHash) {
		return true, nil
	}
	log.Printf("embeddings: checksum mismatch for %s (expected %s got %s), re-downloading", path, expectedHash, sum)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("remove corrupted file: %w", err)
	}
	return false, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func buildFileURL(opts ModelOptions, file ModelFile) string {
	remote := strings.TrimSpace(file.RemotePath)
	if remote == "" {
		remote = file.LocalPath
	}

	if strings.HasPrefix(remote, "http://") || strings.HasPrefix(remote, "https://") {
		return remote
	}

	repo := strings.TrimSpace(opts.Repository)
	rev := strings.Trim(strings.TrimSpace(opts.Revision), "/")
	if repo == "" {
		repo = defaultRepo
	}
	if rev == "" {
		rev = defaultRevision
	}

	remote = strings.TrimPrefix(remote, "/")

	if strings.HasPrefix(repo, "http://") || strings.HasPrefix(repo, "https://") {
		base := strings.TrimSuffix(repo, "/")
		if rev != "" {
			base = fmt.Sprintf("%s/%s", base, rev)
		}
		return fmt.Sprintf("%s/%s", base, remote)
	}

	url := fmt.Sprintf("https://huggingface.co/%s/resolve/%s/%s", strings.Trim(repo, "/"), rev, remote)
	if strings.Contains(remote, "?") {
		return url
	}
	return url + "?download=1"
}

func parseModelFiles(raw string) ([]ModelFile, error) {
	entries := strings.Split(raw, ",")
	files := make([]ModelFile, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.Split(entry, "|")
		local := strings.TrimSpace(parts[0])
		if local == "" {
			return nil, fmt.Errorf("invalid model file entry: %q", entry)
		}
		remote := local
		if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
			remote = strings.TrimSpace(parts[1])
		}
		sha := ""
		if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
			sha = strings.TrimSpace(parts[2])
		}
		files = append(files, ModelFile{LocalPath: local, RemotePath: remote, SHA256: strings.ToLower(sha)})
	}
	return files, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return defaultValue
}
