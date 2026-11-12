package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"era/booru/internal/db"
	embed "era/booru/internal/embeddings"
	"era/booru/internal/processing"
	"era/booru/internal/queue"
	"era/booru/internal/search"
	"era/booru/internal/storage"
	pgvector "github.com/pgvector/pgvector-go"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

func RegisterMediaRoutes(r *gin.Engine, pool *pgxpool.Pool, store *storage.Store, queueClient *river.Client[pgx.Tx], limiter *processing.InferenceLimiter) {
	r.GET("/api/media", listMediaHandler(pool, limiter))
	r.GET("/api/media/previews", listPreviewsHandler(pool, limiter))
	r.GET("/api/media/:id", getMediaHandler(pool))
	r.HEAD("/api/media/:id", headMediaHandler(pool))
	r.GET("/api/media/:id/similar", similarMediaByIDHandler(pool))
	r.POST("/api/media", uploadMediaHandler(pool, store, queueClient))
	r.POST("/api/media/import", func(c *gin.Context) { importMediaHandler(pool, store, queueClient, embed.ActiveProfile())(c) })
	r.GET("/api/embeddings/profile", func(c *gin.Context) { embeddingProfileHandler(embed.ActiveProfile())(c) })
	r.POST("/api/media/:id/tags", updateMediaTagsHandler(pool))
	r.POST("/api/media/:id/dates", updateMediaDatesHandler(pool))
	r.DELETE("/api/media/:id", deleteMediaHandler(pool, queueClient))
	r.StaticFS("/files/originals", http.Dir(store.OriginalsDir()))
	r.StaticFS("/files/previews", http.Dir(store.PreviewsDir()))
}

func listMediaHandler(pool *pgxpool.Pool, limiter *processing.InferenceLimiter) gin.HandlerFunc {
	return listCommon(false, pool, limiter)
}

func listPreviewsHandler(pool *pgxpool.Pool, limiter *processing.InferenceLimiter) gin.HandlerFunc {
	return listCommon(true, pool, limiter)
}

func listCommon(previews bool, pool *pgxpool.Pool, limiter *processing.InferenceLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawQuery := strings.TrimSpace(c.Query("q"))
		vectorQueryParamRaw, hasVectorQueryParam := c.GetQuery("vector_q")
		vectorQueryParam := strings.TrimSpace(vectorQueryParamRaw)
		vectorFlag := c.Query("vector") == "1"
		tagQuery := rawQuery
		vectorQuery := vectorQueryParam
		if vectorFlag && vectorQuery == "" {
			vectorQuery = rawQuery
			if !hasVectorQueryParam {
				tagQuery = ""
			}
		}
		vectorSearch := vectorFlag || vectorQuery != ""
		vectorQuery = strings.TrimSpace(vectorQuery)
		tagQuery = strings.TrimSpace(tagQuery)
		if filterExpr, err := db.ActiveHiddenTagFilterValue(c.Request.Context(), pool); err != nil {
			log.Printf("load hidden tag filter: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		} else if filterExpr != "" {
			if tagQuery != "" {
				tagQuery = strings.TrimSpace(tagQuery + " " + filterExpr)
			} else {
				tagQuery = filterExpr
			}
		}
		pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "60"))
		if err != nil || pageSize < 1 {
			pageSize = 60
		}
		if pageSize > 60 {
			pageSize = 60
		}
		cursor := strings.TrimSpace(c.Query("cursor"))

		var (
			items      []db.Media
			nextCursor string
		)

		if vectorSearch && vectorQuery != "" {
			requestCtx := c.Request.Context()
			vectorName := "vision"
			excludeID := ""
			var vec []float32

			if strings.HasPrefix(vectorQuery, "media:") {
				raw := strings.TrimSpace(strings.TrimPrefix(vectorQuery, "media:"))
				desiredName := "vision"
				if parts := strings.SplitN(raw, ":", 2); len(parts) == 2 {
					if parts[0] != "" {
						desiredName = parts[0]
					}
					raw = parts[1]
				}
				excludeID = strings.TrimSpace(raw)
				if desiredName != "" {
					vectorName = desiredName
				}
				if excludeID != "" {
					loaded, actualName, loadErr := db.GetMediaVector(requestCtx, pool, excludeID, desiredName)
					if loadErr != nil {
						log.Printf("load media vector %s: %v", excludeID, loadErr)
						c.AbortWithStatus(http.StatusInternalServerError)
						return
					}
					if actualName != "" {
						vectorName = actualName
					}
					vec = loaded
				}
			} else {
				var embedErr error
				vec, embedErr = generateTextEmbedding(requestCtx, limiter, vectorQuery)
				if embedErr != nil {
					log.Printf("text embedding %q failed: %v", vectorQuery, embedErr)
					c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": embedErr.Error()})
					return
				}
			}

			if len(vec) == 0 {
				items = []db.Media{}
			} else {
				items, nextCursor, err = search.SimilarMediaByVector(requestCtx, pool, vectorName, vec, pageSize, cursor, excludeID, tagQuery)
				if err != nil {
					log.Printf("vector media search: %v", err)
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}
			}
		} else {
			items, nextCursor, err = search.SearchMedia(c.Request.Context(), pool, tagQuery, pageSize, cursor)
			if err != nil {
				log.Printf("search media: %v", err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
		}
		out := make([]gin.H, len(items))

		for i, mitem := range items {
			key := mitem.ID
			url := mediaURL(key, previews)

			out[i] = gin.H{
				"id":            mitem.ID,
				"original_name": mitem.OriginalName,
				"url":           url,
				"width":         mitem.Width,
				"height":        mitem.Height,
				"format":        mitem.Format,
			}
		}
		c.JSON(http.StatusOK, gin.H{"media": out, "next_cursor": nextCursor})
	}
}

func generateTextEmbedding(ctx context.Context, limiter *processing.InferenceLimiter, text string) ([]float32, error) {
	started := time.Now()
	release, err := limiter.Acquire(ctx, processing.InteractiveInference)
	if err != nil {
		return nil, err
	}
	defer release()
	waited := time.Since(started)
	started = time.Now()
	vector, err := embed.TextEmbedding(text)
	log.Printf("benchmark stage=text_search wait_ms=%.2f inference_ms=%.2f error=%t", float64(waited)/float64(time.Millisecond), float64(time.Since(started))/float64(time.Millisecond), err != nil)
	return vector, err
}

// Existence is independent of processing state and hidden-tag filters, just as
// duplicate detection during upload is. Deleted records are absent from media.
func headMediaHandler(pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		id, ok := idParam(c)
		if !ok {
			return
		}
		exists, err := db.MediaExists(c.Request.Context(), pool, strings.ToLower(id))
		if err != nil {
			log.Printf("check media %s: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if !exists {
			c.Status(http.StatusNotFound)
			return
		}
		c.Status(http.StatusOK)
	}
}

func getMediaHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := idParam(c)
		if !ok {
			log.Printf("invalid media ID: %s", c.Param("id"))
			return
		}

		item, err := db.GetMediaDetail(c.Request.Context(), pool, id)
		if err != nil {
			log.Printf("get media %s: %v", id, err)
			if errors.Is(err, pgx.ErrNoRows) {
				c.AbortWithStatus(http.StatusNotFound)
			} else {
				c.AbortWithStatus(http.StatusInternalServerError)
			}
			return
		}

		url := mediaURL(id, false)
		tags := make([]gin.H, len(item.Tags))
		for i, tag := range item.Tags {
			tags[i] = gin.H{"name": tag.Name, "count": tag.Count}
		}
		dates := make([]gin.H, 0, len(item.Dates))
		for _, date := range item.Dates {
			dates = append(dates, gin.H{"name": date.Name, "value": date.Value})
		}
		embeddings := make([]gin.H, 0, len(item.Embeddings))
		for _, embedding := range item.Embeddings {
			embeddings = append(embeddings, gin.H{"space": embedding.Space, "status": embedding.Status, "error": embedding.Error})
		}

		c.JSON(http.StatusOK, gin.H{
			"status":                 item.Status,
			"error":                  item.Error,
			"embedding_availability": embeddingAvailability(item),
			"id":                     item.ID,
			"original_name":          item.OriginalName,
			"url":                    url,
			"width":                  item.Width,
			"height":                 item.Height,
			"format":                 item.Format,
			"duration":               item.Duration,
			"size":                   item.Size,
			"tags":                   tags,
			"dates":                  dates,
			"embeddings":             embeddings,
		})
	}
}

func similarMediaByIDHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := idParam(c)
		if !ok {
			return
		}
		space := strings.TrimSpace(c.DefaultQuery("space", db.DefaultEmbeddingSpace))
		limit, err := strconv.Atoi(c.DefaultQuery("limit", "5"))
		if err != nil || limit < 1 {
			limit = 5
		}
		if limit > 50 {
			limit = 50
		}
		vector, _, err := db.GetMediaVector(c.Request.Context(), pool, id, space)
		if err != nil {
			log.Printf("load media embedding %s/%s: %v", id, space, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if len(vector) == 0 {
			c.JSON(http.StatusOK, gin.H{"media": []gin.H{}})
			return
		}

		filterExpr, filterErr := db.ActiveHiddenTagFilterValue(c.Request.Context(), pool)
		if filterErr != nil {
			log.Printf("load hidden tag filter: %v", filterErr)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		filterExpr = strings.TrimSpace(filterExpr)

		results, nextCursor, err := search.SimilarMediaByVector(c.Request.Context(), pool, space, vector, limit, c.Query("cursor"), id, filterExpr)
		if err != nil {
			log.Printf("similar media search: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		out := make([]gin.H, 0, len(results))
		for _, item := range results {
			url := mediaURL(item.ID, true)
			out = append(out, gin.H{
				"id":            item.ID,
				"original_name": item.OriginalName,
				"url":           url,
				"width":         item.Width,
				"height":        item.Height,
				"format":        item.Format,
			})
		}

		c.JSON(http.StatusOK, gin.H{"media": out, "next_cursor": nextCursor})
	}
}

func uploadMediaHandler(pool *pgxpool.Pool, store *storage.Store, queueClient *river.Client[pgx.Tx]) gin.HandlerFunc {
	return func(c *gin.Context) {
		reader, err := c.Request.MultipartReader()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "expected multipart/form-data"})
			return
		}
		file, err := reader.NextPart()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing file part"})
			return
		}
		contentType, originalName, err := readMediaFile(file)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		staged, err := store.StageOriginal(c.Request.Context(), file)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer staged.Close()
		if _, err := reader.NextPart(); err != io.EOF {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "expected exactly one file part"})
			return
		}
		completeMediaUpload(c, pool, store, queueClient, staged, contentType, originalName, nil, "")
	}
}

// File installation and rollback cleanup share the per-ID transaction lock with
// deletion and repair. Streaming and multipart validation happen before locking.
func completeMediaUpload(c *gin.Context, pool *pgxpool.Pool, store *storage.Store, queueClient *river.Client[pgx.Tx], staged *storage.StagedOriginal, contentType, originalName string, vector []float32, profile string) {
	ctx := c.Request.Context()
	id := staged.ID
	tx, err := pool.Begin(ctx)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if err := db.LockMediaTx(ctx, tx, id); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	fileExisted, err := staged.Install()
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	committed := false
	commitAttempted := false
	defer func() {
		if !committed && !commitAttempted && !fileExisted {
			_ = store.Remove(id)
		}
	}()
	generation, err := newGeneration()
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	// An explicit upload starts a new generation after a prior deletion.
	if _, err := tx.Exec(ctx, `DELETE FROM deleted_media WHERE id=$1`, id); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	created, err := db.InsertQueuedMedia(ctx, tx, id, generation, originalName, staged.Size, time.Now().UTC())
	if err == nil && created && vector != nil {
		err = db.ImportEmbeddingTx(ctx, tx, id, generation, pgvector.NewVector(vector), profile)
	}
	if err == nil && created {
		err = queue.EnqueueTx(ctx, queueClient, tx, queue.ProcessArgs{Key: id, Generation: generation, ContentType: contentType})
	}
	if err == nil {
		commitAttempted = true
		err = tx.Commit(ctx)
	}
	if err != nil {
		log.Printf("enqueue process %s: %v", id, err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	committed = true
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{"id": id, "existed": !created})
}

func embeddingAvailability(item db.Media) string {
	for _, embedding := range item.Embeddings {
		if embedding.Status == "ready" {
			return "available"
		}
	}
	if !embed.Available() {
		return "unavailable"
	}
	if len(item.Embeddings) > 0 {
		return "available"
	}
	if item.Status == "queued" || item.Status == "processing" || item.Status == "retrying" {
		return "pending"
	}
	return "missing" // Settled media with no job; repair can schedule it.
}

func newGeneration() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func updateMediaTagsHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Tags []string `json:"tags"`
		}
		id, ok := bindIDAndJSON(c, &body)
		if !ok {
			return
		}

		clean := normalizeTags(body.Tags)

		if err := db.SetMediaTags(c.Request.Context(), pool, id, clean); err != nil {
			log.Printf("update media tags %s: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}

func updateMediaDatesHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Dates []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"dates"`
		}
		id, ok := bindIDAndJSON(c, &body)
		if !ok {
			return
		}

		vals := make([]db.DateValue, 0, len(body.Dates))
		for _, d := range body.Dates {
			t, err := time.Parse("2006-01-02", d.Value)
			if err != nil {
				log.Printf("parse date %s: %v", d.Value, err)
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			vals = append(vals, db.DateValue{Name: d.Name, Value: t})
		}

		if err := db.SetMediaDates(c.Request.Context(), pool, id, vals); err != nil {
			log.Printf("update media dates %s: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}

func deleteMediaHandler(pool *pgxpool.Pool, client *river.Client[pgx.Tx]) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := idParam(c)
		if !ok {
			return
		}
		ctx := c.Request.Context()
		tx, err := pool.Begin(ctx)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer tx.Rollback(context.WithoutCancel(ctx))
		if err := db.LockMediaTx(ctx, tx, id); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		generation, err := db.MarkDeletedTx(ctx, tx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			c.Status(http.StatusNoContent)
			return
		}
		if err == nil {
			err = queue.EnqueueTx(ctx, client, tx, queue.CleanupArgs{Key: id, Generation: generation})
		}
		if err == nil {
			err = tx.Commit(ctx)
		}
		if err != nil {
			log.Printf("delete media %s: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func mediaURL(id string, preview bool) string {
	if preview {
		return "/files/previews/" + id
	}
	return "/files/originals/" + id
}
