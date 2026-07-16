package api

import (
	"context"
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

func RegisterMediaRoutes(r *gin.Engine, pool *pgxpool.Pool, store *storage.Store, queueClient *river.Client[pgx.Tx], limiter processing.Limiter) {
	r.GET("/api/media", listMediaHandler(pool, limiter))
	r.GET("/api/media/previews", listPreviewsHandler(pool, limiter))
	r.GET("/api/media/:id", getMediaHandler(pool))
	r.POST("/api/media/similar", similarMediaHandler(pool))
	r.POST("/api/media", uploadMediaHandler(store, queueClient))
	r.POST("/api/media/:id/tags", updateMediaTagsHandler(pool))
	r.POST("/api/media/:id/dates", updateMediaDatesHandler(pool))
	r.POST("/api/media/:id/vectors", updateMediaVectorsHandler(pool))
	r.DELETE("/api/media/:id", deleteMediaHandler(pool, store))
	r.StaticFS("/files/originals", http.Dir(store.OriginalsDir()))
	r.StaticFS("/files/previews", http.Dir(store.PreviewsDir()))
}

func listMediaHandler(pool *pgxpool.Pool, limiter processing.Limiter) gin.HandlerFunc {
	return listCommon(false, pool, limiter)
}

func listPreviewsHandler(pool *pgxpool.Pool, limiter processing.Limiter) gin.HandlerFunc {
	return listCommon(true, pool, limiter)
}

func listCommon(previews bool, pool *pgxpool.Pool, limiter processing.Limiter) gin.HandlerFunc {
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
		page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
		if err != nil || page < 1 {
			page = 1
		}
		pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "60"))
		if err != nil || pageSize < 1 {
			pageSize = 60
		}
		if pageSize > 60 {
			pageSize = 60
		}
		offset := (page - 1) * pageSize

		var (
			items []db.Media
			total int
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
				total = 0
			} else {
				items, total, err = search.SimilarMediaByVector(requestCtx, pool, vectorName, vec, pageSize, offset, excludeID, tagQuery)
				if err != nil {
					log.Printf("vector media search: %v", err)
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}
			}
		} else {
			items, total, err = search.SearchMedia(c.Request.Context(), pool, tagQuery, pageSize, offset)
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
		c.JSON(http.StatusOK, gin.H{"media": out, "total": total})
	}
}

func generateTextEmbedding(ctx context.Context, limiter processing.Limiter, text string) ([]float32, error) {
	release, err := limiter.Acquire(ctx)
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
		vectors := make([]gin.H, 0, len(item.Vectors))
		for _, vector := range item.Vectors {
			vectors = append(vectors, gin.H{"name": vector.Name, "value": vector.Value.Slice()})
		}

		c.JSON(http.StatusOK, gin.H{
			"id":       item.ID,
			"url":      url,
			"width":    item.Width,
			"height":   item.Height,
			"format":   item.Format,
			"duration": item.Duration,
			"size":     item.Size,
			"tags":     tags,
			"dates":    dates,
			"vectors":  vectors,
		})
	}
}

func similarMediaHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Vector  []float32 `json:"vector"`
			Limit   int       `json:"limit"`
			Name    string    `json:"name"`
			Exclude string    `json:"exclude"`
		}
		if err := c.BindJSON(&body); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if len(body.Vector) == 0 {
			c.JSON(http.StatusOK, gin.H{"media": []gin.H{}})
			return
		}

		if body.Name == "" {
			body.Name = "vision"
		}
		if body.Limit <= 0 {
			body.Limit = 5
		}
		if body.Limit > 50 {
			body.Limit = 50
		}

		filterExpr, filterErr := db.ActiveHiddenTagFilterValue(c.Request.Context(), pool)
		if filterErr != nil {
			log.Printf("load hidden tag filter: %v", filterErr)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		filterExpr = strings.TrimSpace(filterExpr)

		results, _, err := search.SimilarMediaByVector(c.Request.Context(), pool, body.Name, body.Vector, body.Limit, 0, body.Exclude, filterExpr)
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

		c.JSON(http.StatusOK, gin.H{"media": out})
	}
}

func uploadMediaHandler(store *storage.Store, queueClient *river.Client[pgx.Tx]) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, existed, err := store.PutHashedOriginal(c.Request.Context(), c.Request.Body)
		if err != nil {
			log.Printf("store upload %s: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		args := queue.ProcessArgs{Key: id, ContentType: c.GetHeader("Content-Type")}
		if err := queue.Enqueue(c.Request.Context(), queueClient, args); err != nil {
			if !existed {
				_ = store.Remove(id)
			}
			log.Printf("enqueue process %s: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		status := http.StatusCreated
		if existed {
			status = http.StatusOK
		}
		c.JSON(status, gin.H{"id": id, "existed": existed})
	}
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

func updateMediaVectorsHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Vectors []struct {
				Name  string    `json:"name"`
				Value []float32 `json:"value"`
			} `json:"vectors"`
		}
		id, ok := bindIDAndJSON(c, &body)
		if !ok {
			return
		}

		vals := make([]db.VectorValue, 0, len(body.Vectors))
		for _, v := range body.Vectors {
			vals = append(vals, db.VectorValue{Name: v.Name, Value: pgvector.NewVector(v.Value)})
		}

		if err := db.SetMediaVectors(c.Request.Context(), pool, id, vals); err != nil {
			log.Printf("update media vectors %s: %v", id, err)
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
