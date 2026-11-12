package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"era/booru/internal/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
)

var ErrDefaultFilter = errors.New("cannot delete the default hidden tag filter")

type Media struct {
	ID           string
	OriginalName string
	Generation   string
	Status       string
	Error        *string
	Format       string
	Width        int
	Height       int
	Duration     *int
	Size         int64
	UploadedAt   time.Time
	Tags         []Tag
	Dates        []DateValue
	Embeddings   []Embedding
}

type Tag struct {
	ID    int64
	Name  string
	Count int
}

type DateValue struct {
	Name  string
	Value time.Time
}

type Embedding struct {
	Space  string
	Status string
	Error  *string
	Value  pgvector.Vector
}

const DefaultEmbeddingSpace = "vision"

type HiddenTagFilter struct {
	ID        int64
	Value     string
	CreatedAt time.Time
}

func Open(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.PostgresDSN)
	if err != nil {
		return nil, err
	}
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
			return err
		}
		return pgxvec.RegisterTypes(ctx, conn)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	var incompatible bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.media') IS NOT NULL AND NOT EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'media' AND column_name = 'generation'
	)`).Scan(&incompatible); err != nil {
		return err
	}
	if incompatible {
		return errors.New("database uses the previous schema; reset the PostgreSQL volume before starting this version")
	}

	_, err := pool.Exec(ctx, `
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS schema_version (
	singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
	version integer NOT NULL
);
INSERT INTO schema_version (singleton, version) VALUES (true, 6)
ON CONFLICT (singleton) DO UPDATE SET version = EXCLUDED.version;

CREATE TABLE IF NOT EXISTS media (
    id text PRIMARY KEY,
	generation text NOT NULL,
	status text NOT NULL CHECK (status IN ('queued', 'processing', 'ready', 'failed')),
	error text,
    format text,
    width integer CHECK (width > 0),
    height integer CHECK (height > 0),
    duration integer CHECK (duration >= 0),
    size bigint NOT NULL CHECK (size >= 0),
    uploaded_at timestamptz NOT NULL DEFAULT now()
);

-- Existing media has no recoverable filename. Preserve its content and leave the name empty.
ALTER TABLE media ADD COLUMN IF NOT EXISTS original_name text NOT NULL DEFAULT '';

-- Upgrade existing installations without resetting their media.
ALTER TABLE media DROP CONSTRAINT IF EXISTS media_status_check;
ALTER TABLE media ADD CONSTRAINT media_status_check
    CHECK (status IN ('queued', 'processing', 'retrying', 'ready', 'failed'));

-- Retained after cleanup so repair cannot resurrect intentionally deleted files.
CREATE TABLE IF NOT EXISTS deleted_media (
    id text PRIMARY KEY,
    generation text NOT NULL,
    cleaned_at timestamptz
);

CREATE TABLE IF NOT EXISTS tag (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL UNIQUE,
    type text NOT NULL DEFAULT 'user_tag' CHECK (type IN ('user_tag', 'meta_tag'))
);

CREATE TABLE IF NOT EXISTS media_tag (
    media_id text NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    tag_id bigint NOT NULL REFERENCES tag(id) ON DELETE CASCADE,
    PRIMARY KEY (media_id, tag_id)
);
CREATE INDEX IF NOT EXISTS media_tag_tag_id_idx ON media_tag(tag_id);

CREATE TABLE IF NOT EXISTS media_date (
    media_id text NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    name text NOT NULL,
    value date NOT NULL,
    PRIMARY KEY (media_id, name)
);

CREATE TABLE IF NOT EXISTS embedding_space (
	id text PRIMARY KEY,
	model text NOT NULL,
	dimensions integer NOT NULL CHECK (dimensions > 0),
	distance text NOT NULL CHECK (distance IN ('cosine', 'inner_product', 'l2'))
);
INSERT INTO embedding_space (id, model, dimensions, distance)
VALUES ('vision', 'siglip2-base', 768, 'inner_product') ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS media_embedding (
    media_id text NOT NULL REFERENCES media(id) ON DELETE CASCADE,
	space_id text NOT NULL REFERENCES embedding_space(id),
	generation text NOT NULL,
	status text NOT NULL CHECK (status IN ('queued', 'processing', 'ready', 'failed')),
	error text,
    value vector,
    profile text NOT NULL DEFAULT '',
    PRIMARY KEY (media_id, space_id)
);
ALTER TABLE media_embedding ADD COLUMN IF NOT EXISTS profile text NOT NULL DEFAULT '';
ALTER TABLE media_embedding DROP CONSTRAINT IF EXISTS media_embedding_status_check;
ALTER TABLE media_embedding ADD CONSTRAINT media_embedding_status_check
    CHECK (status IN ('queued', 'processing', 'retrying', 'ready', 'failed'));
CREATE INDEX IF NOT EXISTS media_embedding_vision_hnsw_idx
    ON media_embedding USING hnsw ((value::vector(768)) vector_ip_ops)
	WHERE space_id = 'vision' AND status = 'ready';

CREATE TABLE IF NOT EXISTS hidden_tag_filter (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    value text NOT NULL UNIQUE CHECK (length(value) <= 1024),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS app_settings (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    active_hidden_filter_id bigint NOT NULL REFERENCES hidden_tag_filter(id)
);
`)
	if err != nil {
		return err
	}
	if err := migrateTagStatistics(ctx, pool); err != nil {
		return err
	}
	return EnsureHiddenTagDefaults(ctx, pool)
}

func IsConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func InsertQueuedMedia(ctx context.Context, tx pgx.Tx, id, generation, originalName string, size int64, uploadedAt time.Time) (bool, error) {
	result, err := tx.Exec(ctx, `INSERT INTO media (id, generation, original_name, status, size, uploaded_at)
VALUES ($1, $2, $3, 'queued', $4, $5) ON CONFLICT (id) DO NOTHING`, id, generation, originalName, size, uploadedAt)
	return err == nil && result.RowsAffected() == 1, err
}

func StartIngestion(ctx context.Context, pool *pgxpool.Pool, id, generation string) (bool, error) {
	result, err := pool.Exec(ctx, `UPDATE media SET status = 'processing', error = NULL
WHERE id = $1 AND generation = $2 AND status IN ('queued', 'processing', 'retrying')`, id, generation)
	return err == nil && result.RowsAffected() == 1, err
}

func RecordIngestionError(ctx context.Context, pool *pgxpool.Pool, id, generation, status string, cause error) error {
	_, err := pool.Exec(ctx, `UPDATE media SET status = $3, error = $4
WHERE id = $1 AND generation = $2 AND status IN ('queued', 'processing', 'retrying')`, id, generation, status, cause.Error())
	return err
}

func CompleteIngestionTx(ctx context.Context, tx pgx.Tx, item Media) error {
	result, err := tx.Exec(ctx, `UPDATE media SET status = 'ready', error = NULL,
format = $3, width = $4, height = $5, duration = $6, size = $7
WHERE id = $1 AND generation = $2 AND status IN ('queued', 'processing', 'retrying')`, item.ID, item.Generation, item.Format, item.Width, item.Height, item.Duration, item.Size)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	var tagID int64
	if err := tx.QueryRow(ctx, `INSERT INTO tag (name) VALUES ('tagme')
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`).Scan(&tagID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO media_tag (media_id, tag_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING`, item.ID, tagID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO media_date (media_id, name, value)
SELECT id, 'upload', uploaded_at FROM media WHERE id = $1 ON CONFLICT DO NOTHING`, item.ID); err != nil {
		return err
	}
	return nil
}

// QueueEmbeddingTx preserves an already-ready vector for this media generation.
func QueueEmbeddingTx(ctx context.Context, tx pgx.Tx, mediaID, generation, space string) (bool, error) {
	var changed bool
	err := withTagStatisticsTx(ctx, tx, mediaID, func() error {
		result, err := tx.Exec(ctx, `INSERT INTO media_embedding (media_id, space_id, generation, status)
VALUES ($1, $2, $3, 'queued')
ON CONFLICT (media_id, space_id) DO UPDATE
SET generation = EXCLUDED.generation, status = 'queued', error = NULL, value = NULL, profile = ''
WHERE media_embedding.generation <> EXCLUDED.generation OR media_embedding.status <> 'ready'`, mediaID, space, generation)
		changed = err == nil && result.RowsAffected() == 1
		return err
	})
	return changed, err
}

// ImportEmbeddingTx records a supplied vector before metadata ingestion completes.
func ImportEmbeddingTx(ctx context.Context, tx pgx.Tx, mediaID, generation string, value pgvector.Vector, profile ...string) error {
	return withTagStatisticsTx(ctx, tx, mediaID, func() error {
		result, err := tx.Exec(ctx, `INSERT INTO media_embedding (media_id, space_id, generation, status, value, profile)
SELECT id, $3, generation, 'ready', $4, $5 FROM media WHERE id=$1 AND generation=$2`, mediaID, generation, DefaultEmbeddingSpace, value, embeddingProfile(profile))
		if err == nil && result.RowsAffected() != 1 {
			return pgx.ErrNoRows
		}
		return err
	})
}

func StartEmbedding(ctx context.Context, pool *pgxpool.Pool, mediaID, generation, space string) (bool, error) {
	result, err := pool.Exec(ctx, `UPDATE media_embedding e SET status = 'processing', error = NULL
FROM media m WHERE e.media_id = m.id AND e.media_id = $1 AND e.space_id = $2
AND e.generation = $3 AND m.generation = $3 AND m.status = 'ready' AND e.status IN ('queued', 'processing', 'retrying')`, mediaID, space, generation)
	return err == nil && result.RowsAffected() == 1, err
}

func RecordEmbeddingError(ctx context.Context, pool *pgxpool.Pool, mediaID, generation, space, status string, cause error) error {
	_, err := pool.Exec(ctx, `UPDATE media_embedding SET status = $4, error = $5
WHERE media_id = $1 AND space_id = $2 AND generation = $3
AND status IN ('queued', 'processing', 'retrying')`, mediaID, space, generation, status, cause.Error())
	return err
}

func CompleteEmbedding(ctx context.Context, pool *pgxpool.Pool, mediaID, generation, space string, value pgvector.Vector, profile ...string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	err = withTagStatisticsTx(ctx, tx, mediaID, func() error {
		result, err := tx.Exec(ctx, `UPDATE media_embedding e SET status = 'ready', error = NULL, value = $4, profile = $5
FROM media m WHERE e.media_id = m.id AND e.media_id = $1 AND e.space_id = $2
AND e.generation = $3 AND m.generation = $3 AND m.status = 'ready' AND e.status IN ('queued', 'processing', 'retrying')`, mediaID, space, generation, value, embeddingProfile(profile))
		if err == nil && result.RowsAffected() != 1 {
			return pgx.ErrNoRows
		}
		return err
	})
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func MediaExists(ctx context.Context, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM media WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

func MediaLifecycle(ctx context.Context, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (generation, status string, err error) {
	err = pool.QueryRow(ctx, `SELECT generation, status FROM media WHERE id = $1`, id).Scan(&generation, &status)
	return
}

func RequeueIngestionTx(ctx context.Context, tx pgx.Tx, id, generation string) error {
	_, err := tx.Exec(ctx, `UPDATE media SET status = 'queued', error = NULL WHERE id = $1 AND generation = $2`, id, generation)
	return err
}

func EmbeddingLifecycle(ctx context.Context, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, mediaID, space string) (status string, err error) {
	err = pool.QueryRow(ctx, `SELECT status FROM media_embedding WHERE media_id = $1 AND space_id = $2`, mediaID, space).Scan(&status)
	return
}

func GetMedia(ctx context.Context, pool *pgxpool.Pool, id string) (Media, error) {
	return getMedia(ctx, pool, id, true)
}

func GetMediaDetail(ctx context.Context, pool *pgxpool.Pool, id string) (Media, error) {
	return getMedia(ctx, pool, id, false)
}

func getMedia(ctx context.Context, pool *pgxpool.Pool, id string, readyOnly bool) (Media, error) {
	var item Media
	err := pool.QueryRow(ctx, `SELECT id, original_name, generation, status, error, COALESCE(format, ''), COALESCE(width, 0), COALESCE(height, 0), duration, size, uploaded_at
FROM media WHERE id = $1 AND (NOT $2 OR status = 'ready')`, id, readyOnly).
		Scan(&item.ID, &item.OriginalName, &item.Generation, &item.Status, &item.Error, &item.Format, &item.Width, &item.Height, &item.Duration, &item.Size, &item.UploadedAt)
	if err != nil {
		return Media{}, err
	}
	tagRows, err := pool.Query(ctx, `SELECT t.id, t.name, count(all_mt.media_id)::int
FROM tag t JOIN media_tag mt ON mt.tag_id = t.id AND mt.media_id = $1
LEFT JOIN media_tag all_mt ON all_mt.tag_id = t.id GROUP BY t.id, t.name ORDER BY t.name`, id)
	if err != nil {
		return Media{}, err
	}
	for tagRows.Next() {
		var tag Tag
		if err := tagRows.Scan(&tag.ID, &tag.Name, &tag.Count); err != nil {
			tagRows.Close()
			return Media{}, err
		}
		item.Tags = append(item.Tags, tag)
	}
	tagRows.Close()
	if err := tagRows.Err(); err != nil {
		return Media{}, err
	}

	dateRows, err := pool.Query(ctx, `SELECT name, value FROM media_date WHERE media_id = $1 ORDER BY name`, id)
	if err != nil {
		return Media{}, err
	}
	for dateRows.Next() {
		var date DateValue
		if err := dateRows.Scan(&date.Name, &date.Value); err != nil {
			dateRows.Close()
			return Media{}, err
		}
		item.Dates = append(item.Dates, date)
	}
	dateRows.Close()
	if err := dateRows.Err(); err != nil {
		return Media{}, err
	}

	embeddingRows, err := pool.Query(ctx, `SELECT space_id, status, error FROM media_embedding WHERE media_id = $1 ORDER BY space_id`, id)
	if err != nil {
		return Media{}, err
	}
	for embeddingRows.Next() {
		var embedding Embedding
		if err := embeddingRows.Scan(&embedding.Space, &embedding.Status, &embedding.Error); err != nil {
			embeddingRows.Close()
			return Media{}, err
		}
		item.Embeddings = append(item.Embeddings, embedding)
	}
	embeddingRows.Close()
	return item, embeddingRows.Err()
}

func ListMedia(ctx context.Context, pool *pgxpool.Pool) ([]Media, error) {
	rows, err := pool.Query(ctx, `SELECT id, original_name, generation, status, error, format, width, height, duration, size, uploaded_at
FROM media WHERE status = 'ready' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Media{}
	byID := map[string]*Media{}
	for rows.Next() {
		var item Media
		if err := rows.Scan(&item.ID, &item.OriginalName, &item.Generation, &item.Status, &item.Error, &item.Format, &item.Width, &item.Height, &item.Duration, &item.Size, &item.UploadedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		byID[items[i].ID] = &items[i]
	}

	tagRows, err := pool.Query(ctx, `SELECT mt.media_id, t.id, t.name, count(all_mt.media_id)::int
FROM media_tag mt JOIN tag t ON t.id = mt.tag_id
LEFT JOIN media_tag all_mt ON all_mt.tag_id = t.id
GROUP BY mt.media_id, t.id, t.name ORDER BY mt.media_id, t.name`)
	if err != nil {
		return nil, err
	}
	for tagRows.Next() {
		var mediaID string
		var tag Tag
		if err := tagRows.Scan(&mediaID, &tag.ID, &tag.Name, &tag.Count); err != nil {
			tagRows.Close()
			return nil, err
		}
		if item := byID[mediaID]; item != nil {
			item.Tags = append(item.Tags, tag)
		}
	}
	tagRows.Close()
	if err := tagRows.Err(); err != nil {
		return nil, err
	}

	dateRows, err := pool.Query(ctx, `SELECT media_id, name, value FROM media_date ORDER BY media_id, name`)
	if err != nil {
		return nil, err
	}
	for dateRows.Next() {
		var mediaID string
		var date DateValue
		if err := dateRows.Scan(&mediaID, &date.Name, &date.Value); err != nil {
			dateRows.Close()
			return nil, err
		}
		if item := byID[mediaID]; item != nil {
			item.Dates = append(item.Dates, date)
		}
	}
	dateRows.Close()
	return items, dateRows.Err()
}

func GetMediaVector(ctx context.Context, pool *pgxpool.Pool, mediaID, space string) ([]float32, string, error) {
	if space == "" {
		space = DefaultEmbeddingSpace
	}
	var vector pgvector.Vector
	err := pool.QueryRow(ctx, `SELECT value FROM media_embedding
WHERE media_id = $1 AND space_id = $2 AND status = 'ready'`, mediaID, space).Scan(&vector)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, space, nil
	}
	if err != nil {
		return nil, "", err
	}
	return vector.Slice(), space, nil
}

func MediaVectorExists(ctx context.Context, pool *pgxpool.Pool, mediaID, name string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(
        SELECT 1 FROM media_embedding WHERE media_id = $1 AND space_id = $2 AND status = 'ready'
    )`, mediaID, name).Scan(&exists)
	return exists, err
}

func SetMediaTags(ctx context.Context, pool *pgxpool.Pool, mediaID string, names []string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	err = withTagStatisticsTx(ctx, tx, mediaID, func() error {
		exists, err := MediaExists(ctx, tx, mediaID)
		if err != nil {
			return err
		}
		if !exists {
			return pgx.ErrNoRows
		}
		return setMediaTagsTx(ctx, tx, mediaID, names)
	})
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func setMediaTagsTx(ctx context.Context, tx pgx.Tx, mediaID string, names []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM media_tag WHERE media_id=$1`, mediaID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `WITH upserted AS (
INSERT INTO tag (name) SELECT DISTINCT unnest($2::text[]) ORDER BY 1
ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name RETURNING id
) INSERT INTO media_tag (media_id,tag_id) SELECT $1,id FROM upserted`, mediaID, names); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `DELETE FROM media_negative_tag n USING media_tag p
WHERE n.media_id=$1 AND p.media_id=n.media_id AND p.tag_id=n.tag_id`, mediaID)
	return err
}

func SetMediaDates(ctx context.Context, pool *pgxpool.Pool, mediaID string, dates []DateValue) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Ingestion holds this lock before updating media and inserting its default
	// upload date. Acquire it first so an override cannot race that insertion or
	// lock media_date before media and deadlock with ingestion.
	if err := LockMediaTx(ctx, tx, mediaID); err != nil {
		return err
	}
	if err := setMediaDatesTx(ctx, tx, mediaID, dates); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func setMediaDatesTx(ctx context.Context, tx pgx.Tx, mediaID string, dates []DateValue) error {
	if _, err := tx.Exec(ctx, `DELETE FROM media_date WHERE media_id = $1`, mediaID); err != nil {
		return err
	}
	names := make([]string, len(dates))
	values := make([]time.Time, len(dates))
	var uploadedAt *time.Time
	for i, date := range dates {
		names[i] = date.Name
		values[i] = date.Value
		if date.Name == "upload" {
			value := date.Value
			uploadedAt = &value
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO media_date (media_id, name, value)
SELECT $1, input.name, input.value FROM unnest($2::text[], $3::timestamptz[]) AS input(name, value)`, mediaID, names, values); err != nil {
		return err
	}
	if uploadedAt != nil {
		if _, err := tx.Exec(ctx, `UPDATE media SET uploaded_at = $2 WHERE id = $1`, mediaID, *uploadedAt); err != nil {
			return err
		}
	}
	return nil
}

func ListTags(ctx context.Context, pool *pgxpool.Pool, prefix string, limit int) ([]Tag, error) {
	args := []any{}
	where := ""
	if prefix != "" {
		args = append(args, prefix+"%")
		where = "WHERE t.name LIKE $1"
	}
	query := `SELECT t.id, t.name, count(mt.media_id)::int FROM tag t
LEFT JOIN media_tag mt ON mt.tag_id = t.id ` + where + ` GROUP BY t.id, t.name`
	if prefix != "" {
		query += ` HAVING count(mt.media_id) > 0`
	}
	query += ` ORDER BY count(mt.media_id) DESC, t.name ASC`
	if limit > 0 {
		args = append(args, limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Tag{}
	for rows.Next() {
		var item Tag
		if err := rows.Scan(&item.ID, &item.Name, &item.Count); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const defaultHiddenFilter = ""

func EnsureHiddenTagDefaults(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `WITH default_filter AS (
    INSERT INTO hidden_tag_filter (value) VALUES ('')
    ON CONFLICT (value) DO UPDATE SET value = EXCLUDED.value RETURNING id
)
INSERT INTO app_settings (singleton, active_hidden_filter_id)
SELECT true, id FROM default_filter ON CONFLICT (singleton) DO NOTHING`)
	return err
}

func ListHiddenTagFilters(ctx context.Context, pool *pgxpool.Pool) ([]HiddenTagFilter, error) {
	rows, err := pool.Query(ctx, `SELECT id, value, created_at FROM hidden_tag_filter ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []HiddenTagFilter{}
	for rows.Next() {
		var item HiddenTagFilter
		if err := rows.Scan(&item.ID, &item.Value, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func CreateHiddenTagFilter(ctx context.Context, pool *pgxpool.Pool, value string) (HiddenTagFilter, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return HiddenTagFilter{}, errors.New("hidden tag filter cannot be empty")
	}
	var item HiddenTagFilter
	err := pool.QueryRow(ctx, `INSERT INTO hidden_tag_filter (value) VALUES ($1) RETURNING id, value, created_at`, value).
		Scan(&item.ID, &item.Value, &item.CreatedAt)
	return item, err
}

func ActiveHiddenTagFilter(ctx context.Context, pool *pgxpool.Pool) (HiddenTagFilter, error) {
	var item HiddenTagFilter
	err := pool.QueryRow(ctx, `SELECT f.id, f.value, f.created_at FROM app_settings s
JOIN hidden_tag_filter f ON f.id = s.active_hidden_filter_id WHERE s.singleton`).
		Scan(&item.ID, &item.Value, &item.CreatedAt)
	return item, err
}

func ActiveHiddenTagFilterValue(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	item, err := ActiveHiddenTagFilter(ctx, pool)
	return item.Value, err
}

func SetActiveHiddenTagFilter(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	result, err := pool.Exec(ctx, `UPDATE app_settings SET active_hidden_filter_id = $1
WHERE singleton AND EXISTS (SELECT 1 FROM hidden_tag_filter WHERE id = $1)`, id)
	if err == nil && result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func DeleteHiddenTagFilter(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var value string
	err = tx.QueryRow(ctx, `SELECT value FROM hidden_tag_filter WHERE id = $1`, id).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if value == defaultHiddenFilter {
		return ErrDefaultFilter
	}
	var defaultID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM hidden_tag_filter WHERE value = ''`).Scan(&defaultID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE app_settings SET active_hidden_filter_id = $1 WHERE active_hidden_filter_id = $2`, defaultID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM hidden_tag_filter WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
