package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"era/booru/internal/config"
	"era/booru/internal/db"
	embed "era/booru/internal/embeddings"
	"era/booru/internal/minio"
	"era/booru/internal/queue"
	embedworker "era/booru/internal/workers/embedworker"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	modelDir := os.Getenv("MODEL_DIR")
	if modelDir == "" {
		modelDir = "ml_models/Siglip2_FP16"
	}
	if err := embed.Load(modelDir); err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	workers := river.NewWorkers()
	river.AddWorker(workers, river.WorkFunc(func(ctx context.Context, job *river.Job[queue.IndexArgs]) error {
		return fmt.Errorf("index job should not run in image embed worker: %s", job.Args.ID)
	}))

	client, err := queue.NewClient(ctx, pool, workers, queue.ClientTypeImageEmbWorker)
	if err != nil {
		log.Fatal(err)
	}

	database, err := db.New(cfg, client, false)
	if err != nil {
		log.Fatal(err)
	}

	m, err := minio.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	river.AddWorker(workers, &embedworker.ImageEmbedWorker{Minio: m, DB: database})

	if err := client.Start(ctx); err != nil {
		log.Fatal(err)
	}
	<-ctx.Done()
	client.Stop(context.Background())
}
