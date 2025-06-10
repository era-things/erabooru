package processing

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
)

type ImageMetadata struct {
	Format string
	Hash   string
	Size   int64
	Width  int
	Height int
}

func GetMetadata(data []byte) (ImageMetadata, error) {
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ImageMetadata{}, err
	}

	metadata := ImageMetadata{
		Format: format,
		Hash:   hash,
		Size:   int64(len(data)),
		Width:  cfg.Width,
		Height: cfg.Height,
	}

	return metadata, nil
}
