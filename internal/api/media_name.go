package api

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// CreateMediaName validates the file extension and generates a unique object name.
func CreateMediaName(filename string) (string, error) {
	extension := filepath.Ext(filename)
	switch strings.ToLower(extension) {
	case ".png", ".jpg", ".jpeg", ".gif":
	default:
		return "", fmt.Errorf("unsupported file format: %s", extension)
	}
	return uuid.New().String() + extension, nil
}
