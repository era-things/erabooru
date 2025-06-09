package main

import (
	"context"
	"era/booru/internal/api"
	"era/booru/internal/config"
	"era/booru/internal/db"
	minio "era/booru/internal/minio"
	"log"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("error loading configuration: %v", err)
	}

	m, err := minio.New(cfg)
	if err != nil {
		log.Fatalf("init minio: %v", err)
	}

	database, err := db.New(cfg)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("watching for new uploads")
	go m.Watch(ctx, database)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), api.CORSMiddleware())

	// Register API routes
	api.RegisterMediaRoutes(r, database, m, cfg)
	api.RegisterStaticRoutes(r)

	r.Run(":8080")
	log.Printf("Server running on http://localhost:8080")

}
