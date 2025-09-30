package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// RequestTextEmbedding enqueues a text embedding job and waits for its result.
func RequestTextEmbedding(ctx context.Context, client *river.Client[pgx.Tx], text string) ([]float32, error) {
	if client == nil {
		return nil, fmt.Errorf("queue client is not configured")
	}

	insertRes, err := client.Insert(ctx, EmbedTextArgs{Text: text}, &river.InsertOpts{Queue: "embed", Priority: 1})
	if err != nil {
		return nil, fmt.Errorf("enqueue text embedding: %w", err)
	}

	jobID := insertRes.Job.ID
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		job, err := client.JobGet(ctx, jobID)
		if err != nil {
			return nil, fmt.Errorf("text embedding job lookup failed: %w", err)
		}

		switch job.State {
		case rivertype.JobStateCompleted:
			output := job.Output()
			if len(output) == 0 {
				return nil, fmt.Errorf("embedding failed: empty output")
			}

			var vec []float32
			if err := json.Unmarshal(output, &vec); err != nil {
				return nil, fmt.Errorf("embedding failed: decode output: %w", err)
			}
			return vec, nil
		case rivertype.JobStateDiscarded:
			msg := "embedding failed"
			if n := len(job.Errors); n > 0 {
				msg = fmt.Sprintf("embedding failed: %s", job.Errors[n-1].Error)
			}
			return nil, errors.New(msg)
		case rivertype.JobStateCancelled:
			return nil, errors.New("embedding job cancelled")
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
