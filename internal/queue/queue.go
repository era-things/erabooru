package queue

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/riverqueue/river/rivertype"
)

var activeUniqueStates = []rivertype.JobState{
	rivertype.JobStateAvailable,
	rivertype.JobStatePending,
	rivertype.JobStateRunning,
	rivertype.JobStateRetryable,
	rivertype.JobStateScheduled,
}

// NewClient creates a River client using the provided database pool and workers.
func NewClient(ctx context.Context, pool *pgxpool.Pool, workers *river.Workers, ingestConcurrency, embedConcurrency int) (*river.Client[pgx.Tx], error) {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return nil, err
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return nil, err
	}
	return river.NewClient(riverpgxv5.New(pool), (&river.Config{
		Queues: map[string]river.QueueConfig{
			"process": {MaxWorkers: ingestConcurrency},
			"embed":   {MaxWorkers: embedConcurrency},
		},
		Workers: workers,
	}).WithDefaults())
}

// Enqueue inserts a job into the default queue.
func Enqueue(ctx context.Context, c *river.Client[pgx.Tx], args river.JobArgs) error {
	return EnqueueTx(ctx, c, nil, args)
}

func EnqueueTx(ctx context.Context, c *river.Client[pgx.Tx], tx pgx.Tx, args river.JobArgs) error {
	opts := &river.InsertOpts{}

	var (
		queueName   string
		priority    int
		setPriority bool
	)

	switch args.(type) {
	case ProcessArgs:
		queueName = "process" // Goes to media worker
	case EmbedArgs:
		queueName = "embed" // Goes to image embed worker
		priority = 2        // lower priority than search embeddings
		setPriority = true
	default:
		queueName = "" // Default queue
	}

	if queueName != "" {
		opts.Queue = queueName
	}
	if setPriority {
		opts.Priority = priority
	}

	var err error
	if tx == nil {
		_, err = c.Insert(ctx, args, opts)
	} else {
		_, err = c.InsertTx(ctx, tx, args, opts)
	}
	return err
}

func WorkerEnqueue(ctx context.Context, args river.JobArgs) error {
	client := river.ClientFromContext[pgx.Tx](ctx)
	if client == nil {
		return river.ErrNotFound
	}
	return Enqueue(ctx, client, args)
}

type ProcessArgs struct {
	Key         string `json:"key"`
	Generation  string `json:"generation"`
	ContentType string `json:"content_type,omitempty"`
}

func (ProcessArgs) Kind() string { return "process_media" }

func (ProcessArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3, UniqueOpts: river.UniqueOpts{
		ByArgs:  true,
		ByState: activeUniqueStates,
	}}
}

type EmbedArgs struct {
	Key        string `json:"key"`
	Generation string `json:"generation"`
	Space      string `json:"space"`
}

func (EmbedArgs) Kind() string { return "embed_media" }

func (EmbedArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3, UniqueOpts: river.UniqueOpts{
		ByArgs:  true,
		ByState: activeUniqueStates,
	}}
}
