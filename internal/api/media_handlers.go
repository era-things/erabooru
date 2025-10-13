package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"era/booru/ent"
	"era/booru/ent/media"
	"era/booru/ent/mediadate"
	"era/booru/ent/mediavector"
	"era/booru/ent/tag"
	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/minio"
	"era/booru/internal/queue"
	"era/booru/internal/search"

	pgvector "github.com/pgvector/pgvector-go"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	mc "github.com/minio/minio-go/v7"
	"github.com/riverqueue/river"
)

func RegisterMediaRoutes(r *gin.Engine, db *ent.Client, m *minio.Client, cfg *config.Config, queueClient *river.Client[pgx.Tx]) {
	r.GET("/api/media", listMediaHandler(cfg, db, queueClient))
	r.GET("/api/media/previews", listPreviewsHandler(cfg, db, queueClient))
	r.GET("/api/media/:id", getMediaHandler(db, m, cfg))
	r.POST("/api/media/similar", similarMediaHandler(db, cfg))
	r.POST("/api/media/upload-url", uploadURLHandler(m, cfg))
	r.POST("/api/media/:id/tags", updateMediaTagsHandler(db))
	r.POST("/api/media/:id/dates", updateMediaDatesHandler(db))
	r.POST("/api/media/:id/vectors", updateMediaVectorsHandler(db))
	r.DELETE("/api/media/:id", deleteMediaHandler(db, m))
}

func bucketForFormat(format, videoBucket, pictureBucket string) string {
	switch format {
	case "mp4", "webm", "avi", "mkv":
		return videoBucket
	default:
		return pictureBucket
	}
}

func listMediaHandler(cfg *config.Config, db *ent.Client, queueClient *river.Client[pgx.Tx]) gin.HandlerFunc {
	return listCommon(cfg.MinioPublicPrefix, cfg.MinioBucket, cfg.MinioBucket, db, queueClient)
}

func listPreviewsHandler(cfg *config.Config, db *ent.Client, queueClient *river.Client[pgx.Tx]) gin.HandlerFunc {
	//for now, image previews are just original full-size images
	return listCommon(cfg.MinioPublicPrefix, cfg.PreviewBucket, cfg.MinioBucket, db, queueClient)
}

func listCommon(minioPrefix string, videoBucket string, pictureBucket string, dbClient *ent.Client, queueClient *river.Client[pgx.Tx]) gin.HandlerFunc {
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
		if filterExpr, err := db.ActiveHiddenTagFilterValue(c.Request.Context(), dbClient); err != nil {
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
			items []*ent.Media
			total int
		)

		if vectorSearch && vectorQuery != "" {
			var includeIDs []string
			if tagQuery != "" {
				includeIDs, err = search.SearchMediaIDs(tagQuery)
				if err != nil {
					log.Printf("filter media ids: %v", err)
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}
				if len(includeIDs) == 0 {
					items = []*ent.Media{}
					total = 0
				}
			}
			if tagQuery != "" && len(includeIDs) == 0 {
				// No candidates left after tag filtering; skip vector ordering.
			} else if queueClient == nil {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "text embedding unavailable"})
				return
			} else {
				ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
				defer cancel()

				vec, err := queue.RequestTextEmbedding(ctx, queueClient, vectorQuery)
				if err != nil {
					log.Printf("text embedding %q failed: %v", vectorQuery, err)
					status := http.StatusServiceUnavailable
					if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
						status = http.StatusGatewayTimeout
					}
					c.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
					return
				}
				if len(vec) == 0 {
					items = []*ent.Media{}
					total = 0
				} else {
					items, total, err = search.SimilarMediaByVector(ctx, dbClient, "vision", vec, pageSize, offset, "", includeIDs)
					if err != nil {
						log.Printf("vector media search: %v", err)
						c.AbortWithStatus(http.StatusInternalServerError)
						return
					}
				}
			}
		} else {
			items, total, err = search.SearchMedia(tagQuery, pageSize, offset)
			if err != nil {
				log.Printf("search media: %v", err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
		}
		out := make([]gin.H, len(items))

		for i, mitem := range items {
			format := mitem.Format
			key := mitem.ID
			bucket := bucketForFormat(format, videoBucket, pictureBucket)

			url := fmt.Sprintf("%s/%s/%s", minioPrefix, bucket, key)

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

func getMediaHandler(db *ent.Client, m *minio.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := idParam(c)
		if !ok {
			log.Printf("invalid media ID: %s", c.Param("id"))
			return
		}

		item, err := db.Media.Query().Where(media.IDEQ(id)).
			WithTags().
			WithDates(func(q *ent.DateQuery) {
				q.WithMediaDates(func(mdq *ent.MediaDateQuery) {
					mdq.Where(mediadate.MediaIDEQ(id))
				})
			}).
			WithVectors(func(q *ent.VectorQuery) {
				q.WithMediaVectors(func(mvq *ent.MediaVectorQuery) {
					mvq.Where(mediavector.MediaIDEQ(id))
				})
			}).
			Only(c.Request.Context())
		if err != nil {
			log.Printf("get media %s: %v", id, err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		stat, err := m.StatObject(c.Request.Context(), m.Bucket, string(id), mc.StatObjectOptions{})
		if err != nil {
			log.Printf("stat object %s: %v", string(id), err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		url := fmt.Sprintf("%s/%s/%s", cfg.MinioPublicPrefix, cfg.MinioBucket, string(id))
		tags := make([]gin.H, len(item.Edges.Tags))
		for i, t := range item.Edges.Tags {
			count, err := db.Tag.Query().Where(tag.IDEQ(t.ID)).QueryMedia().Count(c.Request.Context())
			if err != nil {
				log.Printf("count tag %s: %v", t.Name, err)
				count = 0
			}
			tags[i] = gin.H{"name": t.Name, "count": count}
		}
		dates := make([]gin.H, 0)
		for _, d := range item.Edges.Dates {
			if len(d.Edges.MediaDates) > 0 {
				dates = append(dates, gin.H{
					"name":  d.Name,
					"value": d.Edges.MediaDates[0].Value,
				})
			}
		}
		vectors := make([]gin.H, 0)
		if item.Edges.Vectors != nil {
			for _, vec := range item.Edges.Vectors {
				if vec == nil || vec.Edges.MediaVectors == nil {
					continue
				}
				for _, mv := range vec.Edges.MediaVectors {
					if mv == nil || mv.MediaID != item.ID {
						continue
					}
					data := mv.Value.Slice()
					if len(data) == 0 {
						continue
					}
					copyVec := make([]float32, len(data))
					copy(copyVec, data)
					vectors = append(vectors, gin.H{
						"name":  vec.Name,
						"value": copyVec,
					})
					break
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"id":       item.ID,
			"url":      url,
			"width":    item.Width,
			"height":   item.Height,
			"format":   item.Format,
			"duration": item.Duration,
			"size":     stat.Size,
			"tags":     tags,
			"dates":    dates,
			"vectors":  vectors,
		})
	}
}

func similarMediaHandler(dbClient *ent.Client, cfg *config.Config) gin.HandlerFunc {
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

		includeIDs := []string(nil)
		if filterExpr, filterErr := db.ActiveHiddenTagFilterValue(c.Request.Context(), dbClient); filterErr != nil {
			log.Printf("load hidden tag filter: %v", filterErr)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		} else if filterExpr != "" {
			ids, searchErr := search.SearchMediaIDs(filterExpr)
			if searchErr != nil {
				log.Printf("filter similar media ids: %v", searchErr)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			if len(ids) == 0 {
				c.JSON(http.StatusOK, gin.H{"media": []gin.H{}})
				return
			}
			includeIDs = ids
		}

		results, _, err := search.SimilarMediaByVector(c.Request.Context(), dbClient, body.Name, body.Vector, body.Limit, 0, body.Exclude, includeIDs)
		if err != nil {
			log.Printf("similar media search: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		out := make([]gin.H, 0, len(results))
		for _, item := range results {
			bucket := bucketForFormat(item.Format, cfg.PreviewBucket, cfg.MinioBucket)
			url := fmt.Sprintf("%s/%s/%s", cfg.MinioPublicPrefix, bucket, item.ID)
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

func uploadURLHandler(m *minio.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		type req struct {
			Filename string `json:"filename"`
		}
		var body req
		if err := c.BindJSON(&body); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		url, err := m.PresignedPut(c.Request.Context(), cfg, body.Filename, time.Minute*15)
		if err != nil {
			log.Printf("presign: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, gin.H{"url": url, "object": body.Filename})
	}
}

func updateMediaTagsHandler(dbClient *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Tags []string `json:"tags"`
		}
		id, ok := bindIDAndJSON(c, &body)
		if !ok {
			return
		}

		clean := normalizeTags(body.Tags)

		if err := db.SetMediaTags(c.Request.Context(), dbClient, id, clean); err != nil {
			log.Printf("update media tags %s: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}

func updateMediaDatesHandler(dbClient *ent.Client) gin.HandlerFunc {
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

		if err := db.SetMediaDates(c.Request.Context(), dbClient, id, vals); err != nil {
			log.Printf("update media dates %s: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}

func updateMediaVectorsHandler(dbClient *ent.Client) gin.HandlerFunc {
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

		if err := db.SetMediaVectors(c.Request.Context(), dbClient, id, vals); err != nil {
			log.Printf("update media vectors %s: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}

func deleteMediaHandler(db *ent.Client, m *minio.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := idParam(c)
		if !ok {
			return
		}

		if err := m.RemoveObject(c.Request.Context(), m.Bucket, string(id), mc.RemoveObjectOptions{}); err != nil {
			log.Printf("remove object %s: %v", string(id), err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		if err := db.Media.DeleteOneID(id).Exec(c.Request.Context()); err != nil {
			log.Printf("delete media %s: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.Status(http.StatusOK)
	}
}
