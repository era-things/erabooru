package tagworker

import (
	"context"
	"time"

	"era/booru/internal/db"
	"era/booru/internal/queue"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type RebuildWorker struct {
	river.WorkerDefaults[queue.RebuildTagStatisticsArgs]
	DB *pgxpool.Pool
}

func (*RebuildWorker) Timeout(*river.Job[queue.RebuildTagStatisticsArgs]) time.Duration {
	return 15 * time.Minute
}

func (w *RebuildWorker) Work(ctx context.Context, _ *river.Job[queue.RebuildTagStatisticsArgs]) error {
	return db.RebuildTagStatistics(ctx, w.DB)
}
