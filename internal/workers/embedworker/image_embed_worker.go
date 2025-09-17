package embedworker

import (
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"

	"era/booru/ent"
	"era/booru/internal/db"
	embed "era/booru/internal/embeddings"
	"era/booru/internal/minio"
	"era/booru/internal/queue"

	mc "github.com/minio/minio-go/v7"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/riverqueue/river"
)

// ImageEmbedWorker generates vision embeddings for images.
type ImageEmbedWorker struct {
	river.WorkerDefaults[queue.EmbedArgs]
	Minio *minio.Client
	DB    *ent.Client
}

func (w *ImageEmbedWorker) Work(ctx context.Context, job *river.Job[queue.EmbedArgs]) error {
	log.Printf("Generating embedding for bucket %s, key %s", job.Args.Bucket, job.Args.Key)
	bucket := job.Args.Bucket
	if bucket == "" {
		bucket = w.Minio.Bucket
	}

	obj, err := w.Minio.GetObject(ctx, bucket, job.Args.Key, mc.GetObjectOptions{})
	if err != nil {
		log.Printf("Failed to get object from MinIO: %v", err)
		return err
	}
	defer obj.Close()

	img, _, err := image.Decode(obj)
	if err != nil {
		log.Printf("Failed to decode image: %v", err)
		return err
	}

	vec, err := embed.VisionEmbedding(img)
	if err != nil {
		log.Printf("Failed to generate embedding: %v", err)
		return err
	}

	pgv := pgvector.NewVector(vec)
	if err := db.SetMediaVectors(ctx, w.DB, job.Args.Key, []db.VectorValue{{Name: "vision", Value: pgv}}); err != nil {
		log.Printf("Failed to save embedding to database: %v", err)
		return err
	}

	log.Printf("Successfully generated and saved embedding for key %s", job.Args.Key)
	return nil
}
