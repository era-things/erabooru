package queue

import (
	"context"
	"database/sql"
	"runtime"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
)

// NewClient creates a River client using the provided database and workers.
func NewClient(db *sql.DB, workers *river.Workers) (*river.Client[*sql.Tx], error) {
	cfg := (&river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: runtime.NumCPU()},
		},
		Workers: workers,
	}).WithDefaults()
	return river.NewClient(riverdatabasesql.New(db), cfg)
}

// Enqueue inserts a job into the default queue.
func Enqueue(ctx context.Context, c *river.Client[*sql.Tx], args river.JobArgs) error {
	_, err := c.Insert(ctx, args, nil)
	return err
}

// ProcessArgs describes a media object that needs processing.
type ProcessArgs struct {
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	ContentType string `json:"content_type,omitempty"`
}

func (ProcessArgs) Kind() string { return "process_media" }

// IndexArgs requests a media item to be indexed.
type IndexArgs struct {
	ID string `json:"id"`
}

func (IndexArgs) Kind() string { return "index_media" }
