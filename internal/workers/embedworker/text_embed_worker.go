package embedworker

import (
	"context"
	"log"
	"strings"

	embed "era/booru/internal/embeddings"
	"era/booru/internal/queue"

	"github.com/riverqueue/river"
)

// TextEmbedWorker generates text embeddings using the shared embedding model and
// records the resulting vector on the job output for synchronous retrieval.
type TextEmbedWorker struct {
	river.WorkerDefaults[queue.EmbedTextArgs]
}

func (w *TextEmbedWorker) Work(ctx context.Context, job *river.Job[queue.EmbedTextArgs]) error {
	text := strings.TrimSpace(job.Args.Text)
	if text == "" {
		if err := river.RecordOutput(ctx, struct {
			Vector []float32 `json:"vector"`
			Name   string    `json:"name,omitempty"`
		}{Vector: []float32{}, Name: job.Args.Name}); err != nil {
			log.Printf("record empty text embedding output: %v", err)
			return err
		}
		return nil
	}

	name := job.Args.Name
	if name == "" {
		name = "vision"
	}

	vec, err := embed.TextEmbedding(text)
	if err != nil {
		log.Printf("generate text embedding: %v", err)
		return err
	}

	if err := river.RecordOutput(ctx, struct {
		Vector []float32 `json:"vector"`
		Name   string    `json:"name,omitempty"`
	}{Vector: vec, Name: name}); err != nil {
		log.Printf("record text embedding output: %v", err)
		return err
	}
	return nil
}
