package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"era/booru/internal/config"
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

	srv, err := server.New(ctx, cfg)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}
	defer srv.Close()

	log.Println("Starting Gin server on :8080")
	if err := srv.Run(":8080"); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}
