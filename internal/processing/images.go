package processing

import (
	"bytes"
	"image"
)

type ImageMetadata struct {
	Format string
	Size   int64
	Width  int
	Height int
}

func GetMetadata(data []byte) (ImageMetadata, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ImageMetadata{}, err
	}

	metadata := ImageMetadata{
		Format: format,
		Size:   int64(len(data)),
		Width:  cfg.Width,
		Height: cfg.Height,
	}

	return metadata, nil
}
