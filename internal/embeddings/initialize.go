package embed

import (
	"context"
	"os"
)

// Initialize loads the configured model, downloading it into the model cache
// when MODEL_DIR is not set.
func Initialize(ctx context.Context) error {
	modelDir := os.Getenv("MODEL_DIR")
	if modelDir == "" {
		var err error
		modelDir, err = EnsureModel(ctx, DefaultModelOptionsFromEnv())
		if err != nil {
			return err
		}
	}
	return Load(modelDir)
}
