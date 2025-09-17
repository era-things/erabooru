package indexworker

import (
	"context"
	"log"

	"era/booru/ent"
	"era/booru/ent/media"

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
	log.Printf("Indexing task started for media ID: %s", job.Args.ID)
	mobj, err := w.DB.Media.Query().Where(media.IDEQ(job.Args.ID)).
		WithTags().
		WithDates(func(q *ent.DateQuery) { q.WithMediaDates() }).
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
