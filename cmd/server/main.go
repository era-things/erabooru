package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	"era/booru/internal/api"
	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/ingest"
	minio "era/booru/internal/minio"
	"era/booru/internal/search"
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

	if err := db.InitDefaultProperties(context.Background(), database); err != nil {
		log.Fatalf("init default properties: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("Watching for new uploads")
	go m.Watch(ctx, func(ctx context.Context, object string, contentType string) {
		// Process the uploaded file
		mediaID, err := ingest.Process(ctx, cfg, m, database, object, contentType)
		if err != nil {
			log.Printf("failed to process %s: %v", object, err)
			return
		}

		// Add "tagme" tag if media was successfully processed
		if mediaID != "" {
			tagme, err := db.FindOrCreateTag(ctx, database, "tagme")
			if err != nil {
				log.Printf("error handling tagme tag: %v", err)
				return
			}

			if _, err := database.Media.UpdateOneID(mediaID).
				AddTagIDs(tagme.ID).
				Save(ctx); err != nil {
				log.Printf("error adding tagme tag to media %s: %v", mediaID, err)
			} else {
				log.Printf("added 'tagme' tag to media %s", mediaID)
			}
		}
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
	api.RegisterAdminRoutes(r, database, m, cfg)
	api.RegisterStaticRoutes(r)

	log.Println("Starting Gin server on :8080")
	r.Run(":8080")
	log.Printf("Server running on :8080")

}
