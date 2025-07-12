package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"era/booru/ent"
	"era/booru/ent/media"
	"era/booru/ent/mediadate"
	"era/booru/ent/tag"
	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/minio"
	"era/booru/internal/search"

	pgvector "github.com/pgvector/pgvector-go"

	"github.com/gin-gonic/gin"
	mc "github.com/minio/minio-go/v7"
)

func RegisterMediaRoutes(r *gin.Engine, db *ent.Client, m *minio.Client, cfg *config.Config) {
	r.GET("/api/media", listMediaHandler(cfg))
	r.GET("/api/media/previews", listPreviewsHandler(cfg))
	r.GET("/api/media/:id", getMediaHandler(db, m, cfg))
	r.POST("/api/media/upload-url", uploadURLHandler(m, cfg))
	r.POST("/api/media/:id/tags", updateMediaTagsHandler(db))
	r.POST("/api/media/:id/dates", updateMediaDatesHandler(db))
	r.POST("/api/media/:id/vectors", updateMediaVectorsHandler(db))
	r.DELETE("/api/media/:id", deleteMediaHandler(db, m))
}

func listMediaHandler(cfg *config.Config) gin.HandlerFunc {
	return listCommon(cfg.MinioPublicPrefix, cfg.MinioBucket, cfg.MinioBucket)
}

func listPreviewsHandler(cfg *config.Config) gin.HandlerFunc {
	//for now, image previews are just original full-size images
	return listCommon(cfg.MinioPublicPrefix, cfg.PreviewBucket, cfg.MinioBucket)
}

func listCommon(minioPrefix string, videoBucket string, pictureBucket string) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Query("q")
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
		items, total, err := search.SearchMedia(q, pageSize, offset)
		if err != nil {
			log.Printf("search media: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		out := make([]gin.H, len(items))

		for i, mitem := range items {
			format := mitem.Format
			key := mitem.ID
			var bucket string
			switch format {
			case "mp4", "webm", "avi", "mkv":
				bucket = videoBucket
			default:
				bucket = pictureBucket
			}

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
		})
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
		id, ok := idParam(c)
		if !ok {
			return
		}

		var body struct {
			Tags []string `json:"tags"`
		}
		if err := c.BindJSON(&body); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
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
		id, ok := idParam(c)
		if !ok {
			return
		}

		var body struct {
			Dates []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"dates"`
		}
		if err := c.BindJSON(&body); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
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
		id, ok := idParam(c)
		if !ok {
			return
		}

		var body struct {
			Vectors []struct {
				Name  string    `json:"name"`
				Value []float32 `json:"value"`
			} `json:"vectors"`
		}
		if err := c.BindJSON(&body); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
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
