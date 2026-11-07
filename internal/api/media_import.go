package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	embed "era/booru/internal/embeddings"
	"era/booru/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type importedEmbedding struct {
	Profile string     `json:"profile"`
	Vector  []*float32 `json:"vector"`
}

var errEmbeddingProfile = errors.New("embedding profile does not match the server; fetch /api/embeddings/profile")

func embeddingProfileHandler(profile embed.Profile) gin.HandlerFunc {
	return func(c *gin.Context) {
		if profile.ID == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "embeddings are unavailable"})
			return
		}
		c.JSON(http.StatusOK, profile)
	}
}

func readImportedEmbedding(reader io.Reader, profile embed.Profile) ([]float32, error) {
	// Decode one bounded JSON document before accepting any media bytes.
	data, err := io.ReadAll(io.LimitReader(reader, 64*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 64*1024 {
		return nil, fmt.Errorf("embedding metadata exceeds 64 KiB")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var input importedEmbedding
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("invalid embedding JSON: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("expected one embedding JSON document")
	}
	if input.Profile != profile.ID {
		return nil, errEmbeddingProfile
	}
	if len(input.Vector) != profile.Dimensions {
		return nil, fmt.Errorf("expected %d vector dimensions", profile.Dimensions)
	}
	vector := make([]float32, len(input.Vector))
	var squared float64
	for i, number := range input.Vector {
		if number == nil {
			return nil, fmt.Errorf("vector values must be numbers")
		}
		v := *number
		vector[i] = v
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil, fmt.Errorf("vector values must be finite")
		}
		squared += float64(v) * float64(v)
	}
	if squared == 0 {
		return nil, fmt.Errorf("vector must have nonzero norm")
	}
	norm := math.Sqrt(squared)
	for i, v := range vector {
		vector[i] = float32(float64(v) / norm)
	}
	return vector, nil
}

func readImportMetadata(reader *multipart.Reader, profile embed.Profile) ([]float32, *multipart.Part, string, error) {
	part, err := reader.NextPart()
	if err != nil {
		return nil, nil, "", fmt.Errorf("missing embedding part")
	}
	if part.FormName() != "embedding" {
		return nil, nil, "", fmt.Errorf("first part must be embedding JSON")
	}
	vector, err := readImportedEmbedding(part, profile)
	if err != nil {
		return nil, nil, "", err
	}
	part, err = reader.NextPart()
	if err != nil {
		return nil, nil, "", fmt.Errorf("missing file part")
	}
	if part.FormName() != "file" {
		return nil, nil, "", fmt.Errorf("second part must be file")
	}
	contentType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
	if err != nil || (!strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "video/")) {
		return nil, nil, "", fmt.Errorf("file part requires an image/* or video/* Content-Type")
	}
	return vector, part, contentType, nil
}

func importMediaHandler(pool *pgxpool.Pool, store *storage.Store, client *river.Client[pgx.Tx], profile embed.Profile) gin.HandlerFunc {
	return func(c *gin.Context) {
		if profile.ID == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "embeddings are unavailable"})
			return
		}
		reader, err := c.Request.MultipartReader()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expected multipart/form-data"})
			return
		}
		vector, file, contentType, err := readImportMetadata(reader, profile)
		if err != nil {
			status := http.StatusUnprocessableEntity
			if errors.Is(err, errEmbeddingProfile) {
				status = http.StatusConflict
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		id, existed, err := store.PutHashedOriginal(c.Request.Context(), file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store media"})
			return
		}
		if _, err := reader.NextPart(); err != io.EOF {
			if !existed {
				_ = store.Remove(id)
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "expected exactly embedding and file parts"})
			return
		}
		completeMediaUpload(c, pool, store, client, id, existed, contentType, vector)
	}
}
