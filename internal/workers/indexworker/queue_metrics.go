package indexworker

import (
	"context"
	"errors"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

const (
	indexQueueName     = "index"
	indexQueuePageSize = 512
)

var indexQueueActiveStates = []rivertype.JobState{
	rivertype.JobStateAvailable,
	rivertype.JobStatePending,
	rivertype.JobStateRetryable,
	rivertype.JobStateRunning,
	rivertype.JobStateScheduled,
}

func logIndexQueueDepth(ctx context.Context, message string) {
	remaining, err := countActiveIndexJobs(ctx)
	if err != nil {
		log.Printf("%s; unable to determine remaining index jobs: %v", message, err)
		return
	}

	log.Printf("%s (%d index jobs remaining)", message, remaining)
}

func countActiveIndexJobs(ctx context.Context) (int, error) {
	client := river.ClientFromContext[pgx.Tx](ctx)
	if client == nil {
		return 0, errors.New("river client missing from context")
	}

	params := river.NewJobListParams().
		Queues(indexQueueName).
		States(indexQueueActiveStates...).
		OrderBy(river.JobListOrderByID, river.SortOrderAsc).
		First(indexQueuePageSize)

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
