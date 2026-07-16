package processing

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	_ "golang.org/x/image/webp"
)

type ImageMetadata struct {
	Format string
	Width  int
	Height int
}

func GetMetadata(r io.Reader) (ImageMetadata, error) {
	cfg, format, err := image.DecodeConfig(r)
	if err != nil {
		return ImageMetadata{}, err
	}

	metadata := ImageMetadata{
		Format: format,
		Width:  cfg.Width,
		Height: cfg.Height,
	}

	return metadata, nil
}
