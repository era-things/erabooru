package main

import (
	"context"
	"database/sql"
	"log"
	"os/signal"
	"syscall"

	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/minio"
	"era/booru/internal/queue"
	qworkers "era/booru/internal/queue/workers"

	"github.com/riverqueue/river"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	workers := river.NewWorkers()
	dbpool, err := sql.Open("postgres", cfg.PostgresDSN)
	if err != nil {
		log.Fatal(err)
	}

	client, err := queue.NewClient(dbpool, workers)
	if err != nil {
		log.Fatal(err)
	}

	database, err := db.New(cfg, client)
	if err != nil {
		log.Fatal(err)
	}

	m, err := minio.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	river.AddWorker(workers, &qworkers.ProcessWorker{Minio: m, DB: database, Cfg: cfg, Queue: client})

	if err := client.Start(ctx); err != nil {
		log.Fatal(err)
	}

	<-ctx.Done()
	client.Stop(context.Background())
}
