package embedworker

import (
	"context"
	"fmt"
	"log"

	embed "era/booru/internal/embeddings"
	"era/booru/internal/queue"

	"github.com/riverqueue/river"
)

// TextEmbedWorker generates embeddings for text queries.
type TextEmbedWorker struct {
	river.WorkerDefaults[queue.EmbedTextArgs]
}

func (w *TextEmbedWorker) Work(ctx context.Context, job *river.Job[queue.EmbedTextArgs]) error {
	vec, err := embed.TextEmbedding(job.Args.Text)
	if err != nil {
		log.Printf("Failed to generate text embedding: %v", err)
		return err
	}

	copyVec := make([]float32, len(vec))
	copy(copyVec, vec)

	if err := river.RecordOutput(ctx, copyVec); err != nil {
		log.Printf("Failed to record text embedding output: %v", err)
		return err
	}

	logEmbedQueueDepth(ctx, fmt.Sprintf("Generated text embedding for job %d", job.ID))

	return nil
}
