package db

import (
	"context"
	"errors"
	"time"

	"era/booru/internal/tagmodel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

func embeddingProfile(profile []string) string {
	if len(profile) == 1 {
		return profile[0]
	}
	return ""
}

const dirtyClassifierSQL = `INSERT INTO tag_classifier(tag_id,space_id,profile,due_at)
 VALUES($1,$2,$3,now()+interval '3 seconds')
 ON CONFLICT(tag_id,space_id,profile) DO UPDATE SET revision=tag_classifier.revision+1,
 due_at=excluded.due_at,last_error=NULL`

// Seed new models and refresh the contrast pool at most once every six hours.
// A manual rebuild also captures its target revisions in this transaction.
func scheduleClassifiers(ctx context.Context, tx pgx.Tx, rebuild bool) error {
	_, err := tx.Exec(ctx, `INSERT INTO tag_classifier(tag_id,space_id,profile)
 SELECT tag_id,space_id,profile FROM tag_statistics WHERE space_id=$1 AND profile<>''
 AND positive_count >= $2 ORDER BY tag_id,space_id,profile ON CONFLICT DO NOTHING`, DefaultEmbeddingSpace, tagmodel.MinimumPositives)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE tag_classifier c SET revision=revision+1, due_at=now(),last_error=NULL,
 rebuild_target=CASE WHEN $1 THEN revision+1 ELSE rebuild_target END
 WHERE $1 OR EXISTS(SELECT FROM tag_statistics s WHERE (s.tag_id,s.space_id,s.profile)=(c.tag_id,c.space_id,c.profile)
 AND s.positive_count >= $2)`, rebuild, tagmodel.MinimumPositives)
	return err
}

func RefreshClassifierPool(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var due bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT FROM classifier_pool_dirty) AND pool_refreshed_at <= now()-interval '6 hours' FROM tag_statistics_state`).Scan(&due); err != nil || !due {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, tagStatisticsLock); err != nil {
		return err
	}
	res, err := tx.Exec(ctx, `UPDATE tag_statistics_state SET pool_refreshed_at=now()
 WHERE pool_refreshed_at <= now()-interval '6 hours'`)
	if err != nil || res.RowsAffected() == 0 {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM classifier_pool_dirty`); err != nil {
		return err
	}
	if err = scheduleClassifiers(ctx, tx, false); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func BeginClassifierRebuild(ctx context.Context, pool *pgxpool.Pool, id int64, legacyProfile string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Acquire locks in the same order as source writers.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, tagStatisticsLock); err != nil {
		return err
	}
	var started bool
	if err = tx.QueryRow(ctx, `SELECT classifier_rebuild_id IS NOT DISTINCT FROM $1 FROM tag_statistics_state FOR UPDATE`, id).Scan(&started); err != nil || started {
		return err
	}
	if legacyProfile != "" {
		if _, err = tx.Exec(ctx, `UPDATE media_embedding SET profile=$1 WHERE profile='' AND space_id='vision' AND status='ready' AND vector_dims(value)=768`, legacyProfile); err != nil {
			return err
		}
	}
	if err = rebuildTagStatisticsTx(ctx, tx); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM classifier_pool_dirty`); err != nil {
		return err
	}
	if err = scheduleClassifiers(ctx, tx, true); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE tag_statistics_state SET classifier_rebuild_id=$1,pool_refreshed_at=now()`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func FinishClassifierRebuild(ctx context.Context, pool *pgxpool.Pool, id int64) (bool, error) {
	res, err := pool.Exec(ctx, `UPDATE tag_statistics_state SET classifier_rebuilt_at=now()
 WHERE classifier_rebuild_id=$1 AND NOT EXISTS(SELECT FROM tag_classifier WHERE processed<rebuild_target)`, id)
	return err == nil && res.RowsAffected() == 1, err
}

type classifierSnapshot struct {
	tagID, revision int64
	profile         string
	poolRefreshed   time.Time
	data            tagmodel.Dataset
	initial         []float64
}

func (s *classifierSnapshot) read(ctx context.Context, tx pgx.Tx, rebuild bool) error {
	var prev *pgvector.Vector
	var bias *float64
	var count int
	err := tx.QueryRow(ctx, `SELECT c.tag_id,c.profile,c.revision,
 CASE WHEN c.model_version=$3 THEN c.weights END,c.bias,coalesce(s.positive_count,0),(SELECT pool_refreshed_at FROM tag_statistics_state)
 FROM tag_classifier c LEFT JOIN tag_statistics s USING(tag_id,space_id,profile)
 WHERE c.space_id=$1 AND c.processed<c.revision
 AND (CASE WHEN $2 THEN c.processed<c.rebuild_target ELSE c.due_at<=now() END)
 ORDER BY c.due_at,c.tag_id,c.profile LIMIT 1`, DefaultEmbeddingSpace, rebuild, tagmodel.Version).Scan(&s.tagID, &s.profile, &s.revision, &prev, &bias, &count, &s.poolRefreshed)
	if err != nil || count < tagmodel.MinimumPositives {
		return err
	}
	if prev != nil && bias != nil {
		for _, v := range prev.Slice() {
			s.initial = append(s.initial, float64(v))
		}
		s.initial = append(s.initial, *bias)
	}
	// Content-addressed media IDs give a reproducible ordering independent of any
	// classifier. Scan the profile index until enough unlabeled examples qualify.
	rows, err := tx.Query(ctx, `WITH confirmed AS (
 SELECT media_id,0 AS source FROM media_tag WHERE tag_id=$1
 UNION ALL SELECT media_id,1 FROM media_negative_tag WHERE tag_id=$1
 ), weak AS (
 SELECT e.media_id,2 AS source FROM media_embedding e
 WHERE e.space_id=$2 AND e.profile=$3 AND e.status='ready' AND e.value IS NOT NULL
 AND vector_dims(e.value)=768 AND vector_norm(e.value)>0
 AND NOT EXISTS(SELECT FROM confirmed c WHERE c.media_id=e.media_id)
 ORDER BY e.media_id LIMIT $4
 )
 SELECT x.source,e.value FROM (SELECT * FROM confirmed UNION ALL SELECT * FROM weak) x
 JOIN media_embedding e ON e.media_id=x.media_id AND e.space_id=$2 AND e.profile=$3
 WHERE e.status='ready' AND e.value IS NOT NULL AND vector_dims(e.value)=768 AND vector_norm(e.value)>0
 ORDER BY x.source,x.media_id`, s.tagID, DefaultEmbeddingSpace, s.profile, min(tagmodel.WeakLimit, 4*count))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var source int
		var v pgvector.Vector
		if err := rows.Scan(&source, &v); err != nil {
			return err
		}
		if err := s.data.Add(v.Slice(), tagmodel.Source(source)); err != nil {
			return err
		}
	}
	return rows.Err()
}

// FitNextClassifier holds only a worker lease during optimization. Source
// transactions remain independent. Publication compares the captured revision.
func FitNextClassifier(ctx context.Context, pool *pgxpool.Pool, rebuild bool) (bool, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Release()
	const lease int64 = 731926482
	var locked bool
	if err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, lease).Scan(&locked); err != nil || !locked {
		return false, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := conn.Exec(cleanup, `SELECT pg_advisory_unlock($1)`, lease); err != nil {
			conn.Hijack().Close(cleanup)
		}
	}()
	var s classifierSnapshot
	defer s.data.Close()
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	err = s.read(ctx, tx, rebuild)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	var weights *pgvector.Vector
	var bias float64
	if s.data.Eligible() {
		params, fitErr := s.data.Fit(ctx, s.initial)
		if fitErr != nil {
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			_, err = conn.Exec(ctx, `UPDATE tag_classifier SET last_error=$4,due_at=now()+interval '15 minutes'
   WHERE tag_id=$1 AND space_id='vision' AND profile=$2 AND revision=$3`, s.tagID, s.profile, s.revision, fitErr.Error())
			return false, errors.Join(fitErr, err)
		}
		w := make([]float32, tagmodel.Dimensions)
		for j := range w {
			w[j] = float32(params[j])
		}
		v := pgvector.NewVector(w)
		weights = &v
		bias = params[len(w)]
	}
	_, err = conn.Exec(ctx, `UPDATE tag_classifier SET processed=revision,weights=$4,bias=$5,model_version=$6,
 trained_at=now(),positives=$7,rejections=$8,weak=$9,pool_refreshed_at=$10,last_error=NULL
 WHERE tag_id=$1 AND space_id='vision' AND profile=$2 AND revision=$3`, s.tagID, s.profile, s.revision,
		weights, bias, tagmodel.Version, s.data.Counts[0], s.data.Counts[1], s.data.Counts[2], s.poolRefreshed)
	return err == nil, err
}
