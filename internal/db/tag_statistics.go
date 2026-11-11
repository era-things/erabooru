package db

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

// All source mutations share this lock. A rebuild takes it exclusively, so
// readers retain the previous statistics while concurrent edits wait to commit.
const tagStatisticsLock int64 = 731926481

var ErrInvalidTagFeedback = errors.New("invalid tag feedback")

func migrateTagStatistics(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS media_negative_tag (
 media_id text NOT NULL REFERENCES media(id) ON DELETE CASCADE,
 tag_id bigint NOT NULL REFERENCES tag(id) ON DELETE CASCADE,
 PRIMARY KEY (media_id, tag_id)
);
CREATE INDEX IF NOT EXISTS media_negative_tag_tag_id_idx ON media_negative_tag(tag_id);
CREATE TABLE IF NOT EXISTS tag_statistics (
 tag_id bigint NOT NULL REFERENCES tag(id) ON DELETE CASCADE,
 space_id text NOT NULL REFERENCES embedding_space(id),
 positive_sum vector,
 positive_count bigint NOT NULL DEFAULT 0 CHECK (positive_count >= 0),
 negative_sum vector,
 negative_count bigint NOT NULL DEFAULT 0 CHECK (negative_count >= 0),
 PRIMARY KEY (tag_id, space_id)
);
CREATE TABLE IF NOT EXISTS tag_statistics_state (
 singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
 initialized boolean NOT NULL DEFAULT false,
 rebuilt_at timestamptz
);
INSERT INTO tag_statistics_state (singleton) VALUES (true) ON CONFLICT DO NOTHING;`)
	if err != nil {
		return err
	}
	var initialized bool
	if err := pool.QueryRow(ctx, `SELECT initialized FROM tag_statistics_state`).Scan(&initialized); err != nil {
		return err
	}
	if !initialized {
		return RebuildTagStatistics(ctx, pool)
	}
	return nil
}

// RebuildTagStatistics replaces derived data atomically. Source records are
// never changed. Serializing writers avoids losing edits made during a rebuild.
func RebuildTagStatistics(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, tagStatisticsLock); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
CREATE TEMP TABLE rebuilt_tag_statistics ON COMMIT DROP AS
WITH examples AS (
 SELECT media_id, tag_id, true AS positive FROM media_tag
 UNION ALL
 SELECT media_id, tag_id, false AS positive FROM media_negative_tag
)
SELECT x.tag_id, e.space_id,
 sum(e.value) FILTER (WHERE x.positive) AS positive_sum,
 count(*) FILTER (WHERE x.positive) AS positive_count,
 sum(e.value) FILTER (WHERE NOT x.positive) AS negative_sum,
 count(*) FILTER (WHERE NOT x.positive) AS negative_count
FROM examples x JOIN tag t ON t.id=x.tag_id
JOIN media_embedding e ON e.media_id=x.media_id AND e.status='ready' AND e.value IS NOT NULL
WHERE t.name <> 'tagme' AND vector_norm(e.value) > 0
GROUP BY x.tag_id, e.space_id;
DELETE FROM tag_statistics;
INSERT INTO tag_statistics SELECT * FROM rebuilt_tag_statistics;
UPDATE tag_statistics_state SET initialized=true, rebuilt_at=now();`)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type tagExampleKey struct {
	tagID    int64
	space    string
	positive bool
}
type tagExamples map[tagExampleKey][]float32

func readTagExamplesTx(ctx context.Context, tx pgx.Tx, mediaID string) (tagExamples, error) {
	rows, err := tx.Query(ctx, `WITH examples AS (
 SELECT tag_id, true AS positive FROM media_tag WHERE media_id=$1
 UNION ALL SELECT tag_id, false AS positive FROM media_negative_tag WHERE media_id=$1
)
SELECT x.tag_id, e.space_id, x.positive, e.value
FROM examples x JOIN tag t ON t.id=x.tag_id
JOIN media_embedding e ON e.media_id=$1 AND e.status='ready' AND e.value IS NOT NULL
WHERE t.name <> 'tagme' AND vector_norm(e.value)>0`, mediaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := tagExamples{}
	for rows.Next() {
		var key tagExampleKey
		var value pgvector.Vector
		if err := rows.Scan(&key.tagID, &key.space, &key.positive, &value); err != nil {
			return nil, err
		}
		out[key] = value.Slice()
	}
	return out, rows.Err()
}

// withTagStatisticsTx is the single hook for tag edits, feedback, embedding
// changes and deletion. Comparing final states makes repeated edits idempotent.
func withTagStatisticsTx(ctx context.Context, tx pgx.Tx, mediaID string, mutate func() error) error {
	if err := LockMediaTx(ctx, tx, mediaID); err != nil {
		return err
	}
	before, err := readTagExamplesTx(ctx, tx, mediaID)
	if err != nil {
		return err
	}
	if err := mutate(); err != nil {
		return err
	}
	after, err := readTagExamplesTx(ctx, tx, mediaID)
	if err != nil {
		return err
	}
	return updateTagStatisticsTx(ctx, tx, before, after)
}

func updateTagStatisticsTx(ctx context.Context, tx pgx.Tx, before, after tagExamples) error {
	keys := make([]tagExampleKey, 0, len(before)+len(after))
	for k := range before {
		keys = append(keys, k)
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			keys = append(keys, k)
		}
	}
	// Every writer acquires aggregate rows in the same order.
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.tagID != b.tagID {
			return a.tagID < b.tagID
		}
		if a.space != b.space {
			return a.space < b.space
		}
		return !a.positive && b.positive
	})
	for _, k := range keys {
		old, next := before[k], after[k]
		if slices.Equal(old, next) {
			continue
		}
		for _, change := range []struct {
			value []float32
			count int64
		}{{old, -1}, {next, 1}} {
			if len(change.value) == 0 {
				continue
			}
			// The media may have been deleted, but its tag and statistics still exist.
			// Dimensions belong to an embedding space and must not change in place.
			column, countColumn := "negative_sum", "negative_count"
			if k.positive {
				column, countColumn = "positive_sum", "positive_count"
			}
			if change.count > 0 {
				_, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO tag_statistics (tag_id,space_id,%s,%s)
VALUES ($1,$2,$3,1) ON CONFLICT (tag_id,space_id) DO UPDATE SET
%s=COALESCE(tag_statistics.%s + EXCLUDED.%s, EXCLUDED.%s),
%s=tag_statistics.%s+1`, column, countColumn, column, column, column, column, countColumn, countColumn), k.tagID, k.space, pgvector.NewVector(change.value))
				if err != nil {
					return err
				}
			} else {
				_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE tag_statistics SET
%s=CASE WHEN %s=1 THEN NULL ELSE %s-$3 END, %s=%s-1
WHERE tag_id=$1 AND space_id=$2`, column, countColumn, column, countColumn, countColumn), k.tagID, k.space, pgvector.NewVector(change.value))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// SetTagFeedback accepts one tag without overwriting concurrent edits to others.
// Removing a positive tag does not imply negative feedback.
func SetTagFeedback(ctx context.Context, pool *pgxpool.Pool, mediaID, name, action string) error {
	if name == "" || name == "tagme" || (action != "accept" && action != "reject" && action != "clear") {
		return ErrInvalidTagFeedback
	}
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
		var tagID int64
		if err := tx.QueryRow(ctx, `SELECT id FROM tag WHERE name=$1`, name).Scan(&tagID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM media_negative_tag WHERE media_id=$1 AND tag_id=$2`, mediaID, tagID); err != nil {
			return err
		}
		if action == "accept" {
			_, err = tx.Exec(ctx, `INSERT INTO media_tag VALUES ($1,$2) ON CONFLICT DO NOTHING`, mediaID, tagID)
		} else if action == "reject" {
			if _, err = tx.Exec(ctx, `DELETE FROM media_tag WHERE media_id=$1 AND tag_id=$2`, mediaID, tagID); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO media_negative_tag VALUES ($1,$2) ON CONFLICT DO NOTHING`, mediaID, tagID)
		}
		return err
	})
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type RecommendedTag struct {
	Name     string  `json:"name"`
	Score    float64 `json:"score"`
	Examples int64   `json:"examples"`
}

type TagRecommendations struct {
	Recommended    []RecommendedTag `json:"recommended"`
	LowEvidence    []RecommendedTag `json:"low_evidence"`
	Negative       []string         `json:"negative"`
	EmbeddingReady bool             `json:"embedding_ready"`
}

const minimumRecommendationExamples = 5

func RecommendTags(ctx context.Context, pool *pgxpool.Pool, mediaID string) (TagRecommendations, error) {
	out := TagRecommendations{Recommended: []RecommendedTag{}, LowEvidence: []RecommendedTag{}, Negative: []string{}}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	exists, err := MediaExists(ctx, tx, mediaID)
	if err != nil {
		return out, err
	}
	if !exists {
		return out, pgx.ErrNoRows
	}
	rows, err := tx.Query(ctx, `SELECT t.name FROM media_negative_tag n JOIN tag t ON t.id=n.tag_id WHERE n.media_id=$1 AND t.name<>'tagme' ORDER BY t.name`, mediaID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return out, err
		}
		out.Negative = append(out.Negative, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM media_embedding WHERE media_id=$1 AND space_id=$2 AND status='ready' AND vector_norm(value)>0)`, mediaID, DefaultEmbeddingSpace).Scan(&out.EmbeddingReady); err != nil {
		return out, err
	}
	if out.EmbeddingReady {
		rows, err = tx.Query(ctx, `WITH scored AS (
SELECT t.name,s.positive_count,
 (1-(e.value <=> s.positive_sum)) - 0.5 * CASE WHEN s.negative_count>0 AND vector_norm(s.negative_sum)>0
 THEN 1-(e.value <=> s.negative_sum) ELSE 0 END AS score
FROM tag_statistics s JOIN tag t ON t.id=s.tag_id
JOIN media_embedding e ON e.media_id=$1 AND e.space_id=s.space_id AND e.status='ready'
WHERE s.space_id=$2 AND s.positive_count>0 AND vector_norm(s.positive_sum)>0 AND t.name<>'tagme'
AND NOT EXISTS(SELECT 1 FROM media_tag mt WHERE mt.media_id=$1 AND mt.tag_id=t.id)
AND NOT EXISTS(SELECT 1 FROM media_negative_tag n WHERE n.media_id=$1 AND n.tag_id=t.id)
), ranked AS (
 SELECT *, row_number() OVER (
 PARTITION BY (positive_count >= $3) ORDER BY score DESC,name) AS position
 FROM scored WHERE score>0
)
SELECT name,score,positive_count FROM ranked
WHERE position <= CASE WHEN positive_count >= $3 THEN 5 ELSE 10 END
ORDER BY score DESC,name`, mediaID, DefaultEmbeddingSpace, minimumRecommendationExamples)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var tag RecommendedTag
			if err := rows.Scan(&tag.Name, &tag.Score, &tag.Examples); err != nil {
				rows.Close()
				return out, err
			}
			if tag.Examples >= minimumRecommendationExamples {
				out.Recommended = append(out.Recommended, tag)
			} else {
				out.LowEvidence = append(out.LowEvidence, tag)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return out, err
		}
	}
	return out, tx.Commit(ctx)
}
