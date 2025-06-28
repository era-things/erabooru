package main

import (
	"bytes"
	"context"
	"encoding/json"
	"era/booru/ent"
	"era/booru/internal/api"
	"era/booru/internal/config"
	"era/booru/internal/db"
	minio "era/booru/internal/minio"
	"era/booru/internal/processing"
	"era/booru/internal/search"
	"io"
	"net/http"

	mc "github.com/minio/minio-go/v7"

	"log"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("Loading config...")
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("error loading configuration: %v", err)
	}

	log.Println("Opening Bleve index...")
	if err := search.OpenOrCreate(cfg.BlevePath); err != nil {
		log.Fatalf("error initializing search index: %v", err)
	}

	log.Printf("Initializing MinIO client for bucket '%s'", cfg.MinioBucket)
	m, err := minio.New(cfg)
	if err != nil {
		log.Fatalf("init minio: %v", err)
	}

	log.Println("Connecting to PostgreSQL database...")
	database, err := db.New(cfg)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("Watching for new uploads")
	go m.WatchPictures(ctx, func(ctx context.Context, object string) {
		analyzeImage(m, ctx, database, object)
	})
	go m.WatchVideos(ctx, func(ctx context.Context, object string) {
		analyzeVideo(cfg, m, ctx, database, object)
	})

	r := gin.New()
	r.Use(api.GinLogger(), gin.Recovery(), api.CORSMiddleware())

	// Add health check endpoint
	r.GET("/health", func(c *gin.Context) {
		// 204 No Content response
		c.Status(http.StatusNoContent)
	})

	// Register API routes
	api.RegisterMediaRoutes(r, database, m, cfg)
	api.RegisterAdminRoutes(r, database, cfg)
	api.RegisterStaticRoutes(r)

	log.Println("Starting Gin server on :8080")
	r.Run(":8080")
	log.Printf("Server running on :8080")

}

func analyzeImage(m *minio.Client, ctx context.Context, db *ent.Client, object string) {
	rc, err := m.GetObject(ctx, m.Bucket, object, mc.GetObjectOptions{})
	if err != nil {
		log.Printf("get object %s: %v", object, err)
		return
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		log.Printf("read object %s: %v", object, err)
		return
	}

	metadata, err := processing.GetMetadata(data)
	if err != nil {
		log.Printf("get metadata for %s: %v", object, err)
		return
	}

	if _, err := db.Media.Create().
		SetID(object).
		SetFormat(metadata.Format).
		SetWidth(int16(metadata.Width)).
		SetHeight(int16(metadata.Height)).
		Save(ctx); err != nil {
		log.Printf("create media: %v", err)
	} else {
		log.Printf("saved media %s", object)
	}
}

func analyzeVideo(cfg *config.Config, m *minio.Client, ctx context.Context, db *ent.Client, object string) {
	reqBody := struct {
		Bucket string `json:"bucket"`
		Key    string `json:"key"`
	}{Bucket: m.Bucket, Key: object}

	b, _ := json.Marshal(reqBody)
	resp, err := http.Post(cfg.VideoWorkerURL+"/process", "application/json", bytes.NewReader(b))
	if err != nil {
		log.Printf("video worker request: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("video worker status: %s", resp.Status)
		return
	}
	var out struct {
		PreviewKey string `json:"preview_key"`
		Format     string `json:"format"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		Duration   int    `json:"duration"`
		Hash       string `json:"hash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Printf("decode video worker response: %v", err)
		return
	}

	if _, err := db.Media.Create().
		SetID(reqBody.Key).
		SetFormat(out.Format).
		SetWidth(int16(out.Width)).
		SetHeight(int16(out.Height)).
		SetDuration(int16(out.Duration)).
		Save(ctx); err != nil {
		log.Printf("create video media: %v", err)
	} else {
		log.Printf("saved video %s", object)
	}
}
