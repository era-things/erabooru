package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MediaMetadata contains source metadata only. Statistics, classifiers and
// embeddings are derived from these records and the separately stored media.
type MediaMetadata struct {
	ID           string
	OriginalName *string
	UploadedAt   *time.Time
	Tags         []string
	NegativeTags []string
	Dates        []DateValue
}

// ListMediaMetadata reads every media state in one consistent SQL snapshot.
func ListMediaMetadata(ctx context.Context, pool *pgxpool.Pool) ([]MediaMetadata, error) {
	rows, err := pool.Query(ctx, `SELECT m.id, m.original_name, m.uploaded_at,
 ARRAY(SELECT t.name FROM media_tag x JOIN tag t ON t.id=x.tag_id WHERE x.media_id=m.id ORDER BY t.name),
 ARRAY(SELECT t.name FROM media_negative_tag x JOIN tag t ON t.id=x.tag_id WHERE x.media_id=m.id ORDER BY t.name),
 ARRAY(SELECT name FROM media_date WHERE media_id=m.id ORDER BY name),
 ARRAY(SELECT value FROM media_date WHERE media_id=m.id ORDER BY name)
FROM media m ORDER BY m.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []MediaMetadata{}
	for rows.Next() {
		var item MediaMetadata
		var names []string
		var values []time.Time
		if err := rows.Scan(&item.ID, &item.OriginalName, &item.UploadedAt, &item.Tags, &item.NegativeTags, &names, &values); err != nil {
			return nil, err
		}
		item.Dates = make([]DateValue, len(names))
		for i, name := range names {
			item.Dates[i] = DateValue{Name: name, Value: values[i]}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// RestoreMediaMetadata replaces one item's metadata atomically, using the same
// statistics hook as interactive edits. A nil NegativeTags preserves feedback
// when importing older archives that did not contain rejected tags.
func RestoreMediaMetadata(ctx context.Context, pool *pgxpool.Pool, item MediaMetadata) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	err = withTagStatisticsTx(ctx, tx, item.ID, func() error {
		exists, err := MediaExists(ctx, tx, item.ID)
		if err != nil || !exists {
			if err != nil {
				return err
			}
			return pgx.ErrNoRows
		}
		if err := setMediaTagsTx(ctx, tx, item.ID, item.Tags); err != nil {
			return err
		}
		if item.NegativeTags != nil {
			if _, err := tx.Exec(ctx, `DELETE FROM media_negative_tag WHERE media_id=$1`, item.ID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `WITH upserted AS (
 INSERT INTO tag (name) SELECT DISTINCT unnest($2::text[]) ORDER BY 1
 ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name RETURNING id
 ) INSERT INTO media_negative_tag (media_id,tag_id) SELECT $1,id FROM upserted`, item.ID, item.NegativeTags); err != nil {
				return err
			}
		}
		if item.OriginalName != nil {
			if _, err := tx.Exec(ctx, `UPDATE media SET original_name=$2 WHERE id=$1`, item.ID, *item.OriginalName); err != nil {
				return err
			}
		}
		if err := setMediaDatesTx(ctx, tx, item.ID, item.Dates); err != nil {
			return err
		}
		if item.UploadedAt != nil {
			_, err := tx.Exec(ctx, `UPDATE media SET uploaded_at=$2 WHERE id=$1`, item.ID, *item.UploadedAt)
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
