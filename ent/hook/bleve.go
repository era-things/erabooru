package hook

import (
	"context"

	"era/booru/ent"
	"era/booru/internal/search"
)

// SyncBleve returns a hook that keeps the Bleve index in sync with
// PostgreSQL metadata for Media entities.
func SyncBleve() ent.Hook {
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

			switch {
			case mv.Op().Is(ent.OpDelete | ent.OpDeleteOne):
				if id, ok := mv.ID(); ok {
					if derr := search.DeleteMedia(id); derr != nil {
						return nil, derr
					}
				}
			default:
				media, ok := v.(*ent.Media)
				if !ok {
					id, ok := mv.ID()
					if !ok {
						return v, nil
					}
					media, err = mv.Client().Media.Get(ctx, id)
					if err != nil {
						return nil, err
					}
				}
				if ierr := search.IndexMedia(media); ierr != nil {
					return nil, ierr
				}
			}
			return v, nil
		})
	}
}
