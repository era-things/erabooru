package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"era/booru/internal/queue"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

type embedJobOutput struct {
	Vector []float32 `json:"vector"`
	Name   string    `json:"name,omitempty"`
}

// requestTextEmbedding enqueues a synchronous text embedding job and waits for
// its completion, returning the produced vector and its associated vector name.
func requestTextEmbedding(ctx context.Context, client *river.Client[pgx.Tx], text string) ([]float32, string, error) {
	res, err := queue.Insert(ctx, client, queue.EmbedTextArgs{Text: text, Name: "vision"}, nil)
	if err != nil {
		return nil, "", err
	}
	if res == nil || res.Job == nil {
		return nil, "", fmt.Errorf("embedding job insert returned no job")
	}

	jobID := res.Job.ID
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-ticker.C:
			job, err := client.JobGet(ctx, jobID)
			if err != nil {
				if errors.Is(err, rivertype.ErrNotFound) {
					return nil, "", fmt.Errorf("embedding job %d not found", jobID)
				}
				return nil, "", err
			}

			switch job.State {
			case rivertype.JobStateCompleted:
				output := job.Output()
				var payload embedJobOutput
				if len(output) > 0 {
					if err := json.Unmarshal(output, &payload); err != nil {
						return nil, "", fmt.Errorf("decode embedding output: %w", err)
					}
				}
				if _, err := client.JobDelete(ctx, jobID); err != nil && !errors.Is(err, rivertype.ErrNotFound) {
					return nil, "", fmt.Errorf("cleanup embedding job: %w", err)
				}
				return payload.Vector, payload.Name, nil
			case rivertype.JobStateDiscarded:
				return nil, "", fmt.Errorf("embedding job %d discarded", jobID)
			}
		}
	}
}
