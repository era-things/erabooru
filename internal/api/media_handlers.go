package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"era/booru/internal/db"
	embed "era/booru/internal/embeddings"
	"era/booru/internal/processing"
	"era/booru/internal/queue"
	"era/booru/internal/search"
	"era/booru/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

func RegisterMediaRoutes(r *gin.Engine, pool *pgxpool.Pool, store *storage.Store, queueClient *river.Client[pgx.Tx], limiter *processing.InferenceLimiter) {
	r.GET("/api/media", listMediaHandler(pool, limiter))
	r.GET("/api/media/previews", listPreviewsHandler(pool, limiter))
	r.GET("/api/media/:id", getMediaHandler(pool))
	r.GET("/api/media/:id/similar", similarMediaByIDHandler(pool))
	r.POST("/api/media", uploadMediaHandler(pool, store, queueClient))
	r.POST("/api/media/:id/tags", updateMediaTagsHandler(pool))
	r.POST("/api/media/:id/dates", updateMediaDatesHandler(pool))
	r.DELETE("/api/media/:id", deleteMediaHandler(pool, store))
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
				"id":     mitem.ID,
				"url":    url,
				"width":  mitem.Width,
				"height": mitem.Height,
				"format": mitem.Format,
			}
		}
		c.JSON(http.StatusOK, gin.H{"media": out, "next_cursor": nextCursor})
	}
}

func generateTextEmbedding(ctx context.Context, limiter *processing.InferenceLimiter, text string) ([]float32, error) {
	release, err := limiter.Acquire(ctx, processing.InteractiveInference)
	if err != nil {
		return nil, err
	}
	defer release()
	return embed.TextEmbedding(text)
}

func getMediaHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := idParam(c)
		if !ok {
			log.Printf("invalid media ID: %s", c.Param("id"))
			return
		}

		item, err := db.GetMedia(c.Request.Context(), pool, id)
		if err != nil {
			log.Printf("get media %s: %v", id, err)
			c.AbortWithStatus(http.StatusNotFound)
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
			embeddings = append(embeddings, gin.H{"space": embedding.Space, "status": embedding.Status})
		}

		c.JSON(http.StatusOK, gin.H{
			"id":         item.ID,
			"url":        url,
			"width":      item.Width,
			"height":     item.Height,
			"format":     item.Format,
			"duration":   item.Duration,
			"size":       item.Size,
			"tags":       tags,
			"dates":      dates,
			"embeddings": embeddings,
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
				"id":     item.ID,
				"url":    url,
				"width":  item.Width,
				"height": item.Height,
				"format": item.Format,
			})
		}

		c.JSON(http.StatusOK, gin.H{"media": out, "next_cursor": nextCursor})
	}
}

func uploadMediaHandler(pool *pgxpool.Pool, store *storage.Store, queueClient *river.Client[pgx.Tx]) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, fileExisted, err := store.PutHashedOriginal(c.Request.Context(), c.Request.Body)
		if err != nil {
			log.Printf("store upload %s: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		stat, err := os.Stat(store.OriginalPath(id))
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		generation, err := newGeneration()
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		tx, err := pool.Begin(c.Request.Context())
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer tx.Rollback(c.Request.Context())
		created, err := db.InsertQueuedMedia(c.Request.Context(), tx, id, generation, stat.Size(), time.Now().UTC())
		if err == nil && created {
			err = queue.EnqueueTx(c.Request.Context(), queueClient, tx, queue.ProcessArgs{
				Key: id, Generation: generation, ContentType: c.GetHeader("Content-Type"),
			})
		}
		if err == nil {
			err = tx.Commit(c.Request.Context())
		}
		if err != nil {
			if !fileExisted {
				_ = store.Remove(id)
			}
			log.Printf("enqueue process %s: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		status := http.StatusCreated
		if !created {
			status = http.StatusOK
		}
		c.JSON(status, gin.H{"id": id, "existed": !created})
	}
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

func deleteMediaHandler(pool *pgxpool.Pool, store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := idParam(c)
		if !ok {
			return
		}

		if err := db.DeleteMedia(c.Request.Context(), pool, id); err != nil {
			log.Printf("delete media %s: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if err := store.Remove(id); err != nil {
			log.Printf("cleanup media files %s: %v", id, err)
		}

		c.Status(http.StatusOK)
	}
}

func mediaURL(id string, preview bool) string {
	if preview {
		return "/files/previews/" + id
	}
	return "/files/originals/" + id
}
