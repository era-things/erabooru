package main

import (
	"context"
	"era/booru/internal/api"
	"era/booru/internal/config"
	"era/booru/internal/db"
	minio "era/booru/internal/minio"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
)

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept")
		c.Header("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

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
	r.Use(gin.Logger(), gin.Recovery(), corsMiddleware())

	// Register API routes
	api.RegisterMediaRoutes(r, database, m, cfg)
	api.RegisterStaticRoutes(r)

	r.Run(":8080")
	log.Printf("Server running on http://localhost:8080")

}
