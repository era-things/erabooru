package indexworker

import (
	"context"
	"log"

	"era/booru/ent"
	"era/booru/ent/media"
	"era/booru/ent/mediavector"

	"era/booru/internal/queue"
	"era/booru/internal/search"

	"github.com/riverqueue/river"
)

// IndexWorker updates the Bleve index for a media item.
type IndexWorker struct {
	river.WorkerDefaults[queue.IndexArgs]
	DB *ent.Client
}

func (w *IndexWorker) Work(ctx context.Context, job *river.Job[queue.IndexArgs]) error {
	mobj, err := w.DB.Media.Query().Where(media.IDEQ(job.Args.ID)).
		WithTags().
		WithDates(func(q *ent.DateQuery) { q.WithMediaDates() }).
		WithVectors(func(q *ent.VectorQuery) {
			q.WithMediaVectors(func(mvq *ent.MediaVectorQuery) {
				mvq.Where(mediavector.MediaIDEQ(job.Args.ID))
			})
		}).
		Only(ctx)
	if ent.IsNotFound(err) {
		log.Printf("Media with ID %s not found, deleting from index", job.Args.ID)
		return search.DeleteMedia(job.Args.ID)
	}
	if err != nil {
		log.Printf("Error retrieving media with ID %s: %v", job.Args.ID, err)
		return err
	}
	return search.IndexMedia(mobj)
}
