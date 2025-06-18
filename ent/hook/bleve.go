package hook

import (
	"context"

	"era/booru/ent"
	"era/booru/ent/media"
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
				mobj, ok := v.(*ent.Media)
				if !ok {
					id, ok := mv.ID()
					if !ok {
						return v, nil
					}
					mobj, err = mv.Client().Media.Query().Where(media.IDEQ(id)).WithTags().Only(ctx)
					if err != nil {
						return nil, err
					}
				} else {
					// Reload to include tags for indexing
					mobj, err = mv.Client().Media.Query().Where(media.IDEQ(mobj.ID)).WithTags().Only(ctx)
					if err != nil {
						return nil, err
					}
				}
				if ierr := search.IndexMedia(mobj); ierr != nil {
					return nil, ierr
				}
			}
			return v, nil
		})
	}
}
