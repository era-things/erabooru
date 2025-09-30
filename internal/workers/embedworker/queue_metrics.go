package embedworker

import (
	"context"
	"errors"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

const (
	embedQueueName     = "embed"
	embedQueuePageSize = 512
)

var embedQueueActiveStates = []rivertype.JobState{
	rivertype.JobStateAvailable,
	rivertype.JobStatePending,
	rivertype.JobStateRetryable,
	rivertype.JobStateRunning,
	rivertype.JobStateScheduled,
}

func logEmbedQueueDepth(ctx context.Context, message string) {
	remaining, err := countActiveEmbedJobs(ctx)
	if err != nil {
		log.Printf("%s; unable to determine remaining embed jobs: %v", message, err)
		return
	}

	log.Printf("%s (%d embed jobs remaining)", message, remaining)
}

func countActiveEmbedJobs(ctx context.Context) (int, error) {
	client := river.ClientFromContext[pgx.Tx](ctx)
	if client == nil {
		return 0, errors.New("river client missing from context")
	}

	params := river.NewJobListParams().
		Queues(embedQueueName).
		States(embedQueueActiveStates...).
		OrderBy(river.JobListOrderByID, river.SortOrderAsc).
		First(embedQueuePageSize)

	total := 0
	for {
		res, err := client.JobList(ctx, params)
		if err != nil {
			return 0, err
		}

		total += len(res.Jobs)
		if res.LastCursor == nil {
			break
		}

		params = params.After(res.LastCursor)
	}

	return total, nil
}
