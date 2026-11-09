package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"era/booru/internal/config"
	embed "era/booru/internal/embeddings"
	"era/booru/internal/server"
)

func main() {
	log.Println("Loading config...")
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("error loading configuration: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	modelCtx, cancelModel := context.WithTimeout(ctx, 15*time.Minute)
	if err := embed.Initialize(modelCtx); err != nil {
		cancelModel()
		log.Fatalf("initialize embeddings: %v", err)
	}
	cancelModel()

	srv, err := server.New(ctx, cfg)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}
	defer srv.Close()

	log.Println("Starting Gin server on :8080")
	if err := srv.Run(ctx, ":8080"); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}
