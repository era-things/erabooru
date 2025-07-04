package tasks

import (
	"context"

	"era/booru/ent"
	"era/booru/ent/media"
	"era/booru/internal/search"

	"github.com/riverqueue/river"
)

type IndexArgs struct {
	ID string `json:"id"`
}

func (IndexArgs) Kind() string { return "index_media" }

// IndexWorker indexes or removes a media item in Bleve.
type IndexWorker struct {
	river.WorkerDefaults[IndexArgs]
	DB *ent.Client
}

func (w *IndexWorker) Work(ctx context.Context, job *river.Job[IndexArgs]) error {
	mobj, err := w.DB.Media.Query().
		Where(media.IDEQ(job.Args.ID)).
		WithTags().
		WithDates(func(q *ent.DateQuery) { q.WithMediaDates() }).
		Only(ctx)
	if ent.IsNotFound(err) {
		return search.DeleteMedia(job.Args.ID)
	}
	if err != nil {
		return err
	}
	return search.IndexMedia(mobj)
}
