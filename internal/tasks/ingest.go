package tasks

import (
    "context"

    "era/booru/internal/config"
    "era/booru/internal/db"
    "era/booru/internal/ingest"
    "era/booru/internal/minio"
    "era/booru/ent"

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
    _, err = w.DB.Media.UpdateOneID(mediaID).AddTagIDs(tagme.ID).Save(ctx)
    return err
}
