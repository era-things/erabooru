// cmd/media_worker/main.go
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/minio"
	"era/booru/internal/queue"
	qworkers "era/booru/internal/queue/workers"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create pgxpool
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// Create workers
	workers := river.NewWorkers()

	// Create River client using your queue.NewClient (handles migration automatically)
	client, err := queue.NewClient(ctx, pool, workers, queue.ClientTypeMediaWorker)
	if err != nil {
		log.Fatal(err)
	}

	// Now create database with River client
	database, err := db.New(cfg, client)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize MinIO
	m, err := minio.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Register worker
	river.AddWorker(workers, &qworkers.ProcessWorker{
		Minio: m,
		DB:    database,
		Cfg:   cfg,
	})

	river.AddWorker(workers, &qworkers.IndexWorker{
		DB: database,
	})

	// Start processing
	if err := client.Start(ctx); err != nil {
		log.Fatal(err)
	}

	<-ctx.Done()
	client.Stop(context.Background())
}
