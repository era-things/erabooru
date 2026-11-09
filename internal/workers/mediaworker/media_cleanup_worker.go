package mediaworker

import (
	"context"
	"era/booru/internal/db"
	"era/booru/internal/queue"
	"era/booru/internal/storage"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// CleanupWorker retries file removal without letting repair restore deleted media.
type CleanupWorker struct {
	river.WorkerDefaults[queue.CleanupArgs]
	Store *storage.Store
	DB    *pgxpool.Pool
}

func (w *CleanupWorker) Work(ctx context.Context, job *river.Job[queue.CleanupArgs]) error {
	tx, err := w.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if err := db.LockMediaTx(ctx, tx, job.Args.Key); err != nil {
		return err
	}
	var pending bool
	err = tx.QueryRow(ctx, `SELECT cleaned_at IS NULL FROM deleted_media WHERE id=$1 AND generation=$2`, job.Args.Key, job.Args.Generation).Scan(&pending)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	} // Re-uploaded, or superseded deletion.
	if err != nil {
		return err
	}
	if !pending {
		return nil
	}
	if err := w.Store.Remove(job.Args.Key); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE deleted_media SET cleaned_at=now() WHERE id=$1 AND generation=$2`, job.Args.Key, job.Args.Generation); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
