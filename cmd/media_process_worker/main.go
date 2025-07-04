package main

import (
	"context"
	"log"

	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/minio"
	"era/booru/internal/tasks"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	minioClient, err := minio.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	dbClient, err := db.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	workers := river.NewWorkers()
	river.AddWorker(workers, &tasks.AnalyzeWorker{Cfg: cfg, Minio: minioClient, DB: dbClient})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 5},
		},
		Workers: workers,
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := client.Start(ctx); err != nil {
		log.Fatal(err)
	}
	<-ctx.Done()
	_ = client.Stop(context.Background())
}
