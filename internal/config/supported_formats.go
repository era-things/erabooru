package config

var SupportedImageFormats = map[string]bool{
	"jpg":  true,
	"jpeg": true,
	"png":  true,
	"gif":  true,
	"webp": true,
}

var SupportedVideoFormats = map[string]bool{
	"mp4":  true,
	"webm": true,
	"avi":  true,
	"mkv":  true,
}

var SupportedFormats = map[string]bool{}

func init() {
	for k := range SupportedImageFormats {
		SupportedFormats[k] = true
	}
	for k := range SupportedVideoFormats {
		SupportedFormats[k] = true
	}
}
