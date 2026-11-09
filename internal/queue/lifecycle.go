package queue

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

// HasActiveJobTx avoids requeueing application state while a finishing River
// attempt still owns its uniqueness slot. It also lets repair recover rows
// stranded by a process crash after River exhausted the job.
func HasActiveJobTx(ctx context.Context, tx pgx.Tx, args river.JobArgs) (bool, error) {
	var identity map[string]string
	switch a := args.(type) {
	case ProcessArgs:
		identity = map[string]string{"key": a.Key, "generation": a.Generation}
	case EmbedArgs:
		identity = map[string]string{"key": a.Key, "generation": a.Generation, "space": a.Space}
	default:
		panic("unsupported lifecycle job")
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return false, err
	}
	var active bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM river_job
WHERE kind=$1 AND args @> $2::jsonb
AND state IN ('available', 'pending', 'running', 'retryable', 'scheduled'))`, args.Kind(), encoded).Scan(&active)
	return active, err
}
