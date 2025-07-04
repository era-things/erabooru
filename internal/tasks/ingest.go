package tasks

import (
	"context"

	"era/booru/ent"
	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/ingest"
	"era/booru/internal/minio"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

type AnalyzeArgs struct {
	Object      string `json:"object"`
	ContentType string `json:"content_type"`
}

func (AnalyzeArgs) Kind() string { return "analyze_media" }

// AnalyzeWorker processes a new media object using ingest.Process.
type AnalyzeWorker struct {
	river.WorkerDefaults[AnalyzeArgs]

	Cfg   *config.Config
	Minio *minio.Client
	DB    *ent.Client
}

func (w *AnalyzeWorker) Work(ctx context.Context, job *river.Job[AnalyzeArgs]) error {
	mediaID, err := ingest.Process(ctx, w.Cfg, w.Minio, w.DB, job.Args.Object, job.Args.ContentType)
	if err != nil || mediaID == "" {
		return err
	}

	tagme, err := db.FindOrCreateTag(ctx, w.DB, "tagme")
	if err != nil {
		return err
	}
	if _, err := w.DB.Media.UpdateOneID(mediaID).AddTagIDs(tagme.ID).Save(ctx); err != nil {
		return err
	}

	client := river.ClientFromContext[pgx.Tx](ctx)
	if client != nil {
		if _, err := client.Insert(ctx, IndexArgs{ID: mediaID}, nil); err != nil {
			return err
		}
	}

	return nil
}
