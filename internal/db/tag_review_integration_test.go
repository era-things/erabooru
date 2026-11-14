package db

import (
	"context"
	"testing"
	"time"
)

func TestTagReviewRevisionCommitsAndRollback(t *testing.T) {
	pool := tagTestPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a := tagTestMedia(t, pool, "review-revision-a", nil)
	b := tagTestMedia(t, pool, "review-revision-b", nil)
	revision := func() int64 {
		t.Helper()
		var value int64
		if err := pool.QueryRow(ctx, `SELECT revision FROM tag_review_state`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	before := revision()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for range 2 {
		if _, err := tx.Exec(ctx, `UPDATE media SET original_name=original_name||'x' WHERE id=$1`, a); err != nil {
			t.Fatal(err)
		}
	}
	if revision() != before {
		t.Fatal("uncommitted edit changed cache revision")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if revision() != before+1 {
		t.Fatal("transaction must invalidate exactly once")
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE media SET original_name='rollback' WHERE id=$1`, a); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if revision() != before+1 {
		t.Fatal("rolled-back edit changed cache revision")
	}
	// Writers acquire the revision only after their source writes. Independent
	// commits must both succeed and must not lose an invalidation.
	left, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer left.Rollback(ctx)
	right, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer right.Rollback(ctx)
	if _, err := left.Exec(ctx, `UPDATE media SET original_name='left' WHERE id=$1`, a); err != nil {
		t.Fatal(err)
	}
	if _, err := right.Exec(ctx, `UPDATE media SET original_name='right' WHERE id=$1`, b); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- left.Commit(ctx) }()
	if err := right.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if revision() != before+3 {
		t.Fatal("concurrent commit invalidation was lost")
	}
}
