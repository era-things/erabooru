package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"era/booru/internal/config"
	"era/booru/internal/minio"
	mc "github.com/minio/minio-go/v7"
)

type req struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	minioClient, err := minio.New(cfg)
	if err != nil {
		log.Fatalf("minio: %v", err)
	}

	http.HandleFunc("/process", func(w http.ResponseWriter, r *http.Request) {
		var p req
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Key == "" {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if p.Bucket == "" {
			p.Bucket = cfg.MinioBucket
		}

		// ─── build internal URL FFmpeg can read ─────────────────────────────
		scheme := "http://"
		if cfg.MinioSSL {
			scheme = "https://"
		}
		src := fmt.Sprintf("%s%s/%s/%s", scheme,
			cfg.MinioInternalEndpoint,
			strings.TrimPrefix(p.Bucket, "/"),
			strings.TrimPrefix(p.Key, "/"))

		// ─── probe metadata using ffprobe ─────────────────────────────
		probeOut, err := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", "-show_format", src).Output()
		if err != nil {
			logErr(w, "ffprobe", err)
			return
		}
		var probe struct {
			Streams []struct {
				Width     int    `json:"width"`
				Height    int    `json:"height"`
				Duration  string `json:"duration"`
				CodecType string `json:"codec_type"`
			} `json:"streams"`
			Format struct {
				FormatName string `json:"format_name"`
				Duration   string `json:"duration"`
			} `json:"format"`
		}
		if err := json.Unmarshal(probeOut, &probe); err != nil {
			logErr(w, "probe decode", err)
			return
		}
		width, height := 0, 0
		duration := 0
		for _, s := range probe.Streams {
			if s.CodecType == "video" {
				if s.Width > width {
					width = s.Width
				}
				if s.Height > height {
					height = s.Height
				}
				if s.Duration != "" {
					if f, err := strconv.ParseFloat(s.Duration, 64); err == nil {
						if int(f+0.5) > duration {
							duration = int(f + 0.5)
						}
					}
				}
			}
		}
		if duration == 0 && probe.Format.Duration != "" {
			if f, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
				duration = int(f + 0.5)
			}
		}
		format := probe.Format.FormatName
		supported := []string{"mp4", "webm", "avi", "mkv"}
		for _, f := range supported {
			if strings.Contains(format, f) {
				format = f
				break
			}
		}
		if idx := strings.Index(format, ","); idx != -1 {
			format = format[:idx]
		}

		// ─── run ffmpeg, stream JPEG to stdout ──────────────────────────────
		cmd := exec.Command("ffmpeg",
			"-ss", "00:00:02", "-i", src, "-y", "-loglevel", "error",
			"-vframes", "1", "-vf", "scale=320:-2",
			"-q:v", "3", "-f", "image2", "pipe:1")

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logErr(w, "pipe", err)
			return
		}

		if err := cmd.Start(); err != nil {
			logErr(w, "start", err)
			return
		}

		// ─── upload directly to MinIO ───────────────────────────────────────
		previewKey := strings.TrimSuffix(path.Base(p.Key), path.Ext(p.Key)) + ".jpg"
		_, err = minioClient.PutPreviewJpeg(
			r.Context(),
			previewKey,
			stdout,
		)

		if err != nil {
			logErr(w, "upload", err)
			return
		}

		if err := cmd.Wait(); err != nil {
			logErr(w, "ffmpeg wait", err)
			return
		}

		// ─── hash object ─────────────────────────────────────────────
		obj, err := minioClient.GetObject(r.Context(), p.Bucket, p.Key, mc.GetObjectOptions{})
		if err != nil {
			logErr(w, "get object", err)
			return
		}
		defer obj.Close()
		h := sha256.New()
		if _, err := io.Copy(h, obj); err != nil {
			logErr(w, "hash", err)
			return
		}
		hash := hex.EncodeToString(h.Sum(nil))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"preview_key": previewKey,
			"format":      format,
			"width":       width,
			"height":      height,
			"duration":    duration,
			"hash":        hash,
		})
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func logErr(w http.ResponseWriter, stage string, err error) {
	log.Printf("[%s] %v", stage, err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{
		"error": stage + " error",
	})
}
