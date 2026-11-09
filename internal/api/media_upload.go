package api

import (
	"fmt"
	"mime"
	"mime/multipart"
	"path"
	"strings"
	"unicode/utf8"
)

// readMediaFile validates the shared file part without buffering its contents.
// Names are metadata only; originals continue to use content IDs for storage.
func readMediaFile(part *multipart.Part) (contentType, originalName string, err error) {
	if part.FormName() != "file" {
		return "", "", fmt.Errorf("expected file part")
	}
	originalName = path.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
	if originalName == "." || originalName == "/" || originalName == ".." ||
		!utf8.ValidString(originalName) || strings.ContainsAny(originalName, "\x00\r\n") {
		return "", "", fmt.Errorf("file part requires an original filename")
	}
	contentType, _, err = mime.ParseMediaType(part.Header.Get("Content-Type"))
	if err != nil || (!strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "video/")) {
		return "", "", fmt.Errorf("file part requires an image/* or video/* Content-Type")
	}
	return contentType, originalName, nil
}
