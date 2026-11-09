package db

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// LockMediaTx serializes publication, deletion, repair, and preview installation
// for one content ID, including when no media row exists yet.
func LockMediaTx(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, id)
	return err
}

func IsDeletedTx(ctx context.Context, tx pgx.Tx, id string) (bool, error) {
	var deleted bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM deleted_media WHERE id=$1)`, id).Scan(&deleted)
	return deleted, err
}

// MarkDeletedTx and the cleanup job must commit together. Keep the marker even
// after cleanup; only an intentional upload may remove it.
func MarkDeletedTx(ctx context.Context, tx pgx.Tx, id string) (string, error) {
	var generation string
	err := tx.QueryRow(ctx, `INSERT INTO deleted_media (id, generation)
SELECT id, generation FROM media WHERE id=$1
ON CONFLICT (id) DO UPDATE SET generation=EXCLUDED.generation, cleaned_at=NULL
RETURNING generation`, id).Scan(&generation)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `SELECT generation FROM deleted_media WHERE id=$1`, id).Scan(&generation)
	}
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `DELETE FROM media WHERE id=$1`, id)
	return generation, err
}
