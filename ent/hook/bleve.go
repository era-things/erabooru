package hook

import (
	"context"
	"log"

	"era/booru/ent"
	"era/booru/internal/queue"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

// SyncBleve returns a hook that keeps the Bleve index in sync with
// PostgreSQL metadata for Media entities.
// Optional: Make SyncBleve hook smarter to batch operations
// In your SyncBleve hook
func SyncBleve(q *river.Client[pgx.Tx]) ent.Hook {
	return func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			v, err := next.Mutate(ctx, m)
			if err != nil {
				return v, err
			}

			mv, ok := m.(*ent.MediaMutation)
			if !ok {
				return v, nil
			}

			mvId, _ := mv.ID()
			// Add debug logging
			log.Printf("SyncBleve hook triggered: Op=%s, ID=%v", mv.Op(), mvId)

			if mv.Op().Is(ent.OpCreate | ent.OpUpdateOne | ent.OpDeleteOne) {
				var id string
				if mv.Op().Is(ent.OpDelete | ent.OpDeleteOne) {
					if v, ok := mv.ID(); ok {
						id = v
					}
				} else {
					if v, ok := v.(*ent.Media); ok {
						id = v.ID
					} else if v, ok := mv.ID(); ok {
						id = v
					}
				}

				if id != "" && q != nil {
					log.Printf("SyncBleve enqueueing index job for ID: %s", id)
					if err := queue.Enqueue(ctx, q, queue.IndexArgs{ID: id}); err != nil {
						return nil, err
					}
				}
			}

			return v, nil
		})
	}
}
