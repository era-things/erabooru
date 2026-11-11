package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"testing"
	"time"

	"era/booru/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

func tagTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to an isolated test database")
	}
	pool, err := Open(context.Background(), &config.Config{PostgresDSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func tagTestVector(x, y float32) pgvector.Vector {
	v := make([]float32, 768)
	v[0] = x
	v[1] = y
	return pgvector.NewVector(v)
}

func tagTestMedia(t *testing.T, pool *pgxpool.Pool, name string, value *pgvector.Vector) string {
	t.Helper()
	ctx := context.Background()
	id := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := InsertQueuedMedia(ctx, tx, id, "generation", "example.png", 10, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := CompleteIngestionTx(ctx, tx, Media{ID: id, Generation: "generation", Format: "png", Width: 10, Height: 10, Size: 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := QueueEmbeddingTx(ctx, tx, id, "generation", DefaultEmbeddingSpace); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if value != nil {
		if err := CompleteEmbedding(ctx, pool, id, "generation", DefaultEmbeddingSpace, *value); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Error(err)
			return
		}
		defer tx.Rollback(ctx)
		if _, err := MarkDeletedTx(ctx, tx, id); err != nil {
			t.Error(err)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			t.Error(err)
		}
		pool.Exec(ctx, `DELETE FROM deleted_media WHERE id=$1`, id)
	})
	return id
}

func TestTagStatisticsLifecycle(t *testing.T) {
	pool := tagTestPool(t)
	ctx := context.Background()
	tag := fmt.Sprintf("stats-%d", time.Now().UnixNano())
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM tag WHERE name=$1`, tag) })
	v1, v2 := tagTestVector(1, 0), tagTestVector(0, 1)
	a := tagTestMedia(t, pool, "stats-a", &v1)
	b := tagTestMedia(t, pool, "stats-b", nil)
	check := func(positive, negative int64, wantPos, wantNeg []float32) {
		t.Helper()
		var pc, nc int64
		var pos, neg *pgvector.Vector
		err := pool.QueryRow(ctx, `SELECT positive_count,negative_count,positive_sum,negative_sum FROM tag_statistics s JOIN tag t ON t.id=s.tag_id WHERE t.name=$1`, tag).Scan(&pc, &nc, &pos, &neg)
		if err != nil {
			t.Fatal(err)
		}
		if pc != positive || nc != negative {
			t.Fatalf("counts=(%d,%d), want (%d,%d)", pc, nc, positive, negative)
		}
		for _, pair := range []struct {
			got  *pgvector.Vector
			want []float32
		}{{pos, wantPos}, {neg, wantNeg}} {
			if pair.want == nil {
				if pair.got != nil {
					t.Fatalf("expected NULL sum, got %v", pair.got)
				}
				continue
			}
			if pair.got == nil || !slices.Equal(pair.got.Slice()[:2], pair.want) {
				t.Fatalf("wrong vector: %v, want first coordinates %v", pair.got, pair.want)
			}
		}
	}
	if err := SetMediaTags(ctx, pool, a, []string{tag, "tagme"}); err != nil {
		t.Fatal(err)
	}
	if err := SetMediaTags(ctx, pool, b, []string{tag}); err != nil {
		t.Fatal(err)
	}
	check(1, 0, []float32{1, 0}, nil)
	if err := CompleteEmbedding(ctx, pool, b, "generation", DefaultEmbeddingSpace, v2); err != nil {
		t.Fatal(err)
	}
	check(2, 0, []float32{1, 1}, nil)
	if err := SetMediaTags(ctx, pool, b, []string{tag, tag}); err != nil {
		t.Fatal(err)
	}
	check(2, 0, []float32{1, 1}, nil)
	for i := 0; i < 2; i++ {
		if err := SetTagFeedback(ctx, pool, b, tag, "reject"); err != nil {
			t.Fatal(err)
		}
	}
	check(1, 1, []float32{1, 0}, []float32{0, 1})
	if err := SetTagFeedback(ctx, pool, b, tag, "clear"); err != nil {
		t.Fatal(err)
	}
	check(1, 0, []float32{1, 0}, nil)
	if err := SetTagFeedback(ctx, pool, b, tag, "reject"); err != nil {
		t.Fatal(err)
	}
	if err := SetMediaTags(ctx, pool, b, []string{tag}); err != nil {
		t.Fatal(err)
	}
	check(2, 0, []float32{1, 1}, nil)
	result, err := RecommendTags(ctx, pool, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Negative) != 0 {
		t.Fatal("manual tag addition retained negative feedback")
	}
	// A failed transaction changes neither source records nor aggregates.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = withTagStatisticsTx(ctx, tx, b, func() error {
		_, err := tx.Exec(ctx, `DELETE FROM media_tag WHERE media_id=$1`, b)
		if err != nil {
			return err
		}
		return errors.New("injected failure")
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}
	tx.Rollback(ctx)
	check(2, 0, []float32{1, 1}, nil)
	// Replacing an embedding removes its old contribution until the new one is ready.
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := LockMediaTx(ctx, tx, b); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE media SET generation='replacement' WHERE id=$1`, b); err != nil {
		t.Fatal(err)
	}
	if _, err := QueueEmbeddingTx(ctx, tx, b, "replacement", DefaultEmbeddingSpace); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	check(1, 0, []float32{1, 0}, nil)
	if err := CompleteEmbedding(ctx, pool, b, "replacement", DefaultEmbeddingSpace, v1); err != nil {
		t.Fatal(err)
	}
	check(2, 0, []float32{2, 0}, nil)
	if err := SetTagFeedback(ctx, pool, b, tag, "reject"); err != nil {
		t.Fatal(err)
	}
	if err := RebuildTagStatistics(ctx, pool); err != nil {
		t.Fatal(err)
	}
	check(1, 1, []float32{1, 0}, []float32{1, 0})
	// Corrupted accumulated data is repaired from the source records.
	if _, err := pool.Exec(ctx, `UPDATE tag_statistics SET positive_count=42 WHERE tag_id=(SELECT id FROM tag WHERE name=$1)`, tag); err != nil {
		t.Fatal(err)
	}
	if err := RebuildTagStatistics(ctx, pool); err != nil {
		t.Fatal(err)
	}
	check(1, 1, []float32{1, 0}, []float32{1, 0})
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MarkDeletedTx(ctx, tx, a); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	check(0, 1, nil, []float32{1, 0})
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tag_statistics s JOIN tag t ON t.id=s.tag_id WHERE t.name='tagme'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("tagme was learned: count=%d err=%v", count, err)
	}
	if err := SetTagFeedback(ctx, pool, b, "tagme", "reject"); !errors.Is(err, ErrInvalidTagFeedback) {
		t.Fatalf("tagme feedback accepted: %v", err)
	}
}

func TestTagRecommendationEvidenceGroups(t *testing.T) {
	pool := tagTestPool(t)
	ctx := context.Background()
	prefix := fmt.Sprintf("evidence-%d-", time.Now().UnixNano())
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM tag WHERE name LIKE $1`, prefix+"%") })
	v := tagTestVector(1, 0)
	target := tagTestMedia(t, pool, "evidence-target", &v)
	tags := make([]string, 6)
	for i := range tags {
		tags[i] = fmt.Sprintf("%sestablished-%d", prefix, i)
	}
	oneShot := prefix + "one-shot"
	var last string
	for i := 0; i < 5; i++ {
		var embedding *pgvector.Vector
		if i < 4 {
			embedding = &v
		}
		id := tagTestMedia(t, pool, "evidence-training", embedding)
		if err := SetMediaTags(ctx, pool, id, tags); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if err := SetTagFeedback(ctx, pool, id, tags[0], "accept"); err != nil {
				t.Fatal(err)
			}
			if err := SetMediaTags(ctx, pool, id, append(append([]string{}, tags...), oneShot)); err != nil {
				t.Fatal(err)
			}
		}
		last = id
	}
	before, err := RecommendTags(ctx, pool, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Recommended) != 0 || len(before.LowEvidence) != 7 {
		t.Fatalf("unready embedding counted as evidence: main=%d tentative=%d", len(before.Recommended), len(before.LowEvidence))
	}
	if err := CompleteEmbedding(ctx, pool, last, "generation", DefaultEmbeddingSpace, v); err != nil {
		t.Fatal(err)
	}
	after, err := RecommendTags(ctx, pool, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Recommended) != 5 || len(after.LowEvidence) != 1 || after.LowEvidence[0].Name != oneShot || after.LowEvidence[0].Examples != 1 {
		t.Fatalf("wrong evidence groups: %+v", after)
	}
	for i, tag := range after.Recommended {
		if tag.Examples != 5 || tag.Name != tags[i] {
			t.Fatalf("wrong count or tie ordering: %+v", after.Recommended)
		}
	}
	if err := SetTagFeedback(ctx, pool, target, oneShot, "reject"); err != nil {
		t.Fatal(err)
	}
	if err := SetTagFeedback(ctx, pool, target, tags[0], "accept"); err != nil {
		t.Fatal(err)
	}
	filtered, err := RecommendTags(ctx, pool, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.LowEvidence) != 0 || len(filtered.Recommended) != 5 {
		t.Fatalf("groups not independently filtered: %+v", filtered)
	}
	for _, tag := range filtered.Recommended {
		if tag.Name == tags[0] || tag.Name == "tagme" {
			t.Fatalf("applied or excluded tag recommended: %s", tag.Name)
		}
	}
	// Opposite visual content yields an empty list instead of filling available slots.
	opposite := tagTestVector(-1, 0)
	unrelated := tagTestMedia(t, pool, "evidence-unrelated", &opposite)
	empty, err := RecommendTags(ctx, pool, unrelated)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Recommended) != 0 || len(empty.LowEvidence) != 0 {
		t.Fatalf("weak matches filled list: %+v", empty)
	}
}

func TestTagRecommendationsAndNegativeFeedback(t *testing.T) {
	pool := tagTestPool(t)
	ctx := context.Background()
	tag := fmt.Sprintf("recommend-%d", time.Now().UnixNano())
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM tag WHERE name=$1`, tag) })
	v := tagTestVector(1, 0)
	a := tagTestMedia(t, pool, "recommend-a", &v)
	b := tagTestMedia(t, pool, "recommend-b", &v)
	c := tagTestMedia(t, pool, "recommend-c", &v)
	if err := SetMediaTags(ctx, pool, a, []string{tag}); err != nil {
		t.Fatal(err)
	}
	score := func(id string) float64 {
		t.Helper()
		result, err := RecommendTags(ctx, pool, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range append(result.Recommended, result.LowEvidence...) {
			if r.Name == "tagme" {
				t.Fatal("tagme recommendation")
			}
			if r.Name == tag {
				return r.Score
			}
		}
		return -1
	}
	before := score(c)
	if math.Abs(before-1) > 1e-6 {
		t.Fatalf("score=%f want 1", before)
	}
	if err := SetTagFeedback(ctx, pool, b, tag, "reject"); err != nil {
		t.Fatal(err)
	}
	if score(b) != -1 {
		t.Fatal("rejected tag was recommended again")
	}
	after := score(c)
	if after >= before || math.Abs(after-0.5) > 1e-6 {
		t.Fatalf("negative feedback did not lower score: %f -> %f", before, after)
	}
	if err := SetTagFeedback(ctx, pool, b, tag, "accept"); err != nil {
		t.Fatal(err)
	}
	if score(b) != -1 {
		t.Fatal("already applied tag was recommended")
	}
	if math.Abs(score(c)-before) > 1e-6 {
		t.Fatal("acceptance did not remove negative contribution")
	}
	// Rebuild must wait for in-flight source edits, then include their committed state.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := LockMediaTx(ctx, tx, c); err != nil {
		t.Fatal(err)
	}
	err = withTagStatisticsTx(ctx, tx, c, func() error {
		_, err := tx.Exec(ctx, `INSERT INTO media_negative_tag SELECT $1,id FROM tag WHERE name=$2`, c, tag)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	rebuildCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	go func() { done <- RebuildTagStatistics(rebuildCtx, pool) }()
	select {
	case err := <-done:
		t.Fatalf("rebuild bypassed pending edit: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var pc, nc int
	if err := pool.QueryRow(ctx, `SELECT positive_count,negative_count FROM tag_statistics s JOIN tag t ON t.id=s.tag_id WHERE t.name=$1`, tag).Scan(&pc, &nc); err != nil || pc != 2 || nc != 1 {
		t.Fatalf("rebuild lost edit: positives=%d negatives=%d err=%v", pc, nc, err)
	}
}
