package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path"
	"strings"

	"era/booru/internal/config"
	"era/booru/internal/minio"
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

	http.HandleFunc("/thumb", func(w http.ResponseWriter, r *http.Request) {
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
		_, err = minioClient.PutPreviewJpeg(
			r.Context(),
			path.Base(p.Key),
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

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"preview_key":%q}`, path.Base(p.Key))
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
