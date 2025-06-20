package api

import (
	"era/booru/internal/config"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// CreateMediaName validates the file extension and generates a unique object name.
func CreateMediaName(filename string) (string, error) {
	extension := filepath.Ext(filename)

	if !config.SupportedFormats[strings.ToLower(extension[1:])] {
		return "", fmt.Errorf("unsupported file format: %s", extension)
	}

	return uuid.New().String() + extension, nil
}
