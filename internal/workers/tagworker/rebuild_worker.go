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

type FitWorker struct {
	river.WorkerDefaults[queue.FitTagsArgs]
	DB *pgxpool.Pool
}

func (*FitWorker) Timeout(*river.Job[queue.FitTagsArgs]) time.Duration { return 15 * time.Minute }

func (w *FitWorker) Work(ctx context.Context, _ *river.Job[queue.FitTagsArgs]) error {
	if err := db.RefreshClassifierPool(ctx, w.DB); err != nil {
		return err
	}
	return fitBatch(ctx, w.DB, false)
}

func fitBatch(ctx context.Context, pool *pgxpool.Pool, rebuild bool) error {
	for range 16 {
		worked, err := db.FitNextClassifier(ctx, pool, rebuild)
		if err != nil || !worked {
			return err
		}
	}
	return nil
}

func (*RebuildWorker) Timeout(*river.Job[queue.RebuildTagStatisticsArgs]) time.Duration {
	return 15 * time.Minute
}

func (w *RebuildWorker) Work(ctx context.Context, job *river.Job[queue.RebuildTagStatisticsArgs]) error {
	if err := db.BeginClassifierRebuild(ctx, w.DB, job.ID, job.Args.LegacyProfile); err != nil {
		return err
	}
	if err := fitBatch(ctx, w.DB, true); err != nil {
		return err
	}
	done, err := db.FinishClassifierRebuild(ctx, w.DB, job.ID)
	if err != nil || done {
		return err
	}
	return river.JobSnooze(time.Second)
}
