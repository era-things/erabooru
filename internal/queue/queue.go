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

type ClientType string

const (
	ClientTypeServer         ClientType = "server"
	ClientTypeMediaWorker    ClientType = "worker"
	ClientTypeImageEmbWorker ClientType = "image_embed_worker"
)

// NewClient creates a River client using the provided database pool and workers.
func NewClient(ctx context.Context, pool *pgxpool.Pool, workers *river.Workers, clientType ClientType) (*river.Client[pgx.Tx], error) {
	// Only run migrations for the server, not workers
	if clientType == ClientTypeServer {
		migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
		if err != nil {
			pool.Close()
			return nil, err
		}
		if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
			pool.Close()
			return nil, err
		}
	}

	cfg := getConfigForClientType(clientType, workers)
	return river.NewClient(riverpgxv5.New(pool), cfg)
}

func getConfigForClientType(clientType ClientType, workers *river.Workers) *river.Config {
	switch clientType {
	case ClientTypeServer:
		return (&river.Config{
			Queues: map[string]river.QueueConfig{
				"index": {MaxWorkers: 3},
			},
			Workers: workers,
		}).WithDefaults()
	case ClientTypeMediaWorker:
		return (&river.Config{
			Queues: map[string]river.QueueConfig{
				"process": {MaxWorkers: 3},
			},
			Workers: workers,
		}).WithDefaults()
	case ClientTypeImageEmbWorker:
		return (&river.Config{
			Queues: map[string]river.QueueConfig{
				"embed": {MaxWorkers: 3},
			},
			Workers: workers,
		}).WithDefaults()
	default:
		return &river.Config{
			Workers: workers,
		}
	}
}

// Enqueue inserts a job into the default queue.
func Enqueue(ctx context.Context, c *river.Client[pgx.Tx], args river.JobArgs) error {
	opts := &river.InsertOpts{}
	if queueName := queueNameForArgs(args); queueName != "" {
		opts.Queue = queueName
	}

	_, err := c.Insert(ctx, args, opts)
	return err
}

// Insert inserts a job and returns the River result while ensuring it is routed
// to the appropriate queue based on the job arguments.
func Insert(ctx context.Context, c *river.Client[pgx.Tx], args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	insertOpts := opts
	if insertOpts == nil {
		insertOpts = &river.InsertOpts{}
	}
	if insertOpts.Queue == "" {
		if queueName := queueNameForArgs(args); queueName != "" {
			insertOpts.Queue = queueName
		}
	}

	return c.Insert(ctx, args, insertOpts)
}

func queueNameForArgs(args river.JobArgs) string {
	switch args.(type) {
	case ProcessArgs:
		return "process"
	case IndexArgs:
		return "index"
	case EmbedArgs, EmbedTextArgs:
		return "embed"
	default:
		return ""
	}
}

func WorkerEnqueue(ctx context.Context, args river.JobArgs) error {
	client := river.ClientFromContext[pgx.Tx](ctx)
	if client == nil {
		return river.ErrNotFound
	}
	return Enqueue(ctx, client, args)
}

type ProcessArgs struct {
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	ContentType string `json:"content_type,omitempty"`
}

func (ProcessArgs) Kind() string { return "process_media" }

type IndexArgs struct {
	ID string `json:"id"`
}

func (IndexArgs) Kind() string { return "index_media" }

type EmbedArgs struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

func (EmbedArgs) Kind() string { return "embed_media" }

type EmbedTextArgs struct {
	Text string `json:"text"`
	Name string `json:"name,omitempty"`
}

func (EmbedTextArgs) Kind() string { return "embed_text" }
