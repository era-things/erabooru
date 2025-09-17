package queue

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"
)

// RegisterEmbedJobStub registers a placeholder embed job worker so clients that
// do not depend on the CGO-enabled embed worker can still enqueue embed jobs.
//
// The stub should never run because non-embed services do not poll the embed
// queue. If it does run, it returns an error to surface the misconfiguration.
func RegisterEmbedJobStub(workers *river.Workers) {
	river.AddWorker(workers, river.WorkFunc(func(ctx context.Context, job *river.Job[EmbedArgs]) error {
		_ = ctx
		return fmt.Errorf("embed job stub executed for key %q; embed worker not registered in this service", job.Args.Key)
	}))
}
