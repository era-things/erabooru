package hook

import (
	"context"
	"database/sql"

	"era/booru/ent"
	"era/booru/internal/queue"
	"github.com/riverqueue/river"
)

// SyncBleve returns a hook that keeps the Bleve index in sync with
// PostgreSQL metadata for Media entities.
func SyncBleve(q *river.Client[*sql.Tx]) ent.Hook {
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
				if err := queue.Enqueue(ctx, q, queue.IndexArgs{ID: id}); err != nil {
					return nil, err
				}
			}
			// indexing is deferred to queue worker
			return v, nil
		})
	}
}
