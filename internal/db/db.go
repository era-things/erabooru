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
	ID         string
	Format     string
	Width      int
	Height     int
	Duration   *int
	Size       int64
	UploadedAt time.Time
	Tags       []Tag
	Dates      []DateValue
	Vectors    []VectorValue
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

type VectorValue struct {
	Name  string
	Value pgvector.Vector
}

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
	_, err := pool.Exec(ctx, `
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS media (
    id text PRIMARY KEY,
    format text NOT NULL,
    width integer NOT NULL CHECK (width > 0),
    height integer NOT NULL CHECK (height > 0),
    duration integer CHECK (duration >= 0),
    size bigint NOT NULL CHECK (size >= 0),
    uploaded_at timestamptz NOT NULL DEFAULT now()
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

CREATE TABLE IF NOT EXISTS media_vector (
    media_id text NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    name text NOT NULL,
    value vector(768) NOT NULL,
    PRIMARY KEY (media_id, name)
);
CREATE INDEX IF NOT EXISTS media_vector_value_hnsw_idx
    ON media_vector USING hnsw (value vector_ip_ops);

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
	return EnsureHiddenTagDefaults(ctx, pool)
}

func IsConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func SaveMedia(ctx context.Context, pool *pgxpool.Pool, item Media) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `INSERT INTO media (id, format, width, height, duration, size, uploaded_at)
VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO NOTHING`,
		item.ID, item.Format, item.Width, item.Height, item.Duration, item.Size, item.UploadedAt)
	if err != nil || result.RowsAffected() == 0 {
		return err
	}
	var tagID int64
	if err := tx.QueryRow(ctx, `INSERT INTO tag (name) VALUES ('tagme')
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`).Scan(&tagID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO media_tag (media_id, tag_id) VALUES ($1, $2)`, item.ID, tagID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO media_date (media_id, name, value) VALUES ($1, 'upload', $2)`, item.ID, item.UploadedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func MediaExists(ctx context.Context, pool *pgxpool.Pool, id string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM media WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

func GetMedia(ctx context.Context, pool *pgxpool.Pool, id string) (Media, error) {
	var item Media
	err := pool.QueryRow(ctx, `SELECT id, format, width, height, duration, size, uploaded_at FROM media WHERE id = $1`, id).
		Scan(&item.ID, &item.Format, &item.Width, &item.Height, &item.Duration, &item.Size, &item.UploadedAt)
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

	vectorRows, err := pool.Query(ctx, `SELECT name, value FROM media_vector WHERE media_id = $1 ORDER BY name`, id)
	if err != nil {
		return Media{}, err
	}
	for vectorRows.Next() {
		var vector VectorValue
		if err := vectorRows.Scan(&vector.Name, &vector.Value); err != nil {
			vectorRows.Close()
			return Media{}, err
		}
		item.Vectors = append(item.Vectors, vector)
	}
	vectorRows.Close()
	return item, vectorRows.Err()
}

func ListMedia(ctx context.Context, pool *pgxpool.Pool) ([]Media, error) {
	rows, err := pool.Query(ctx, `SELECT id FROM media ORDER BY id`)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	items := make([]Media, 0, len(ids))
	for _, id := range ids {
		item, err := GetMedia(ctx, pool, id)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func GetMediaVector(ctx context.Context, pool *pgxpool.Pool, mediaID, desiredName string) ([]float32, string, error) {
	rows, err := pool.Query(ctx, `SELECT name, value FROM media_vector WHERE media_id = $1 ORDER BY (name = $2) DESC, name`, mediaID, desiredName)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, "", rows.Err()
	}
	var name string
	var vector pgvector.Vector
	if err := rows.Scan(&name, &vector); err != nil {
		return nil, "", err
	}
	return vector.Slice(), name, nil
}

func MediaVectorExists(ctx context.Context, pool *pgxpool.Pool, mediaID, name string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(
        SELECT 1 FROM media_vector WHERE media_id = $1 AND name = $2
    )`, mediaID, name).Scan(&exists)
	return exists, err
}

func DeleteMedia(ctx context.Context, pool *pgxpool.Pool, id string) error {
	result, err := pool.Exec(ctx, `DELETE FROM media WHERE id = $1`, id)
	if err == nil && result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func SetMediaTags(ctx context.Context, pool *pgxpool.Pool, mediaID string, names []string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM media_tag WHERE media_id = $1`, mediaID); err != nil {
		return err
	}
	for _, name := range names {
		var id int64
		if err := tx.QueryRow(ctx, `INSERT INTO tag (name) VALUES ($1)
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, name).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO media_tag (media_id, tag_id) VALUES ($1, $2)`, mediaID, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func SetMediaDates(ctx context.Context, pool *pgxpool.Pool, mediaID string, dates []DateValue) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM media_date WHERE media_id = $1`, mediaID); err != nil {
		return err
	}
	for _, date := range dates {
		if _, err := tx.Exec(ctx, `INSERT INTO media_date (media_id, name, value) VALUES ($1, $2, $3)`, mediaID, date.Name, date.Value); err != nil {
			return err
		}
		if date.Name == "upload" {
			if _, err := tx.Exec(ctx, `UPDATE media SET uploaded_at = $2 WHERE id = $1`, mediaID, date.Value); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func SetMediaVectors(ctx context.Context, pool *pgxpool.Pool, mediaID string, vectors []VectorValue) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM media_vector WHERE media_id = $1`, mediaID); err != nil {
		return err
	}
	for _, vector := range vectors {
		if _, err := tx.Exec(ctx, `INSERT INTO media_vector (media_id, name, value) VALUES ($1, $2, $3)`, mediaID, vector.Name, vector.Value); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
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
