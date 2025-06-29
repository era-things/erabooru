package api

import (
	"compress/gzip"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"era/booru/ent"
	"era/booru/ent/media"
	"era/booru/internal/config"
	"era/booru/internal/ingest"
	minio "era/booru/internal/minio"
	"era/booru/internal/search"

	"github.com/gin-gonic/gin"
	mc "github.com/minio/minio-go/v7"
)

// RegisterAdminRoutes registers admin-only endpoints.
func RegisterAdminRoutes(r *gin.Engine, db *ent.Client, m *minio.Client, cfg *config.Config) {
	r.POST("/api/admin/regenerate", regenerateHandler(db, m, cfg))
	r.GET("/api/admin/export-tags", exportTagsHandler(db))
}

func regenerateHandler(db *ent.Client, m *minio.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if err := search.Rebuild(ctx, db, cfg.BlevePath); err != nil {
			log.Printf("regenerate index: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		// Iterate over all objects in the bucket and regenerate missing metadata
		for obj := range m.ListObjects(ctx, m.Bucket, mc.ListObjectsOptions{Recursive: true}) {
			if obj.Err != nil {
				log.Printf("list object %s: %v", obj.Key, obj.Err)
				continue
			}

			exists, err := db.Media.Query().Where(media.IDEQ(obj.Key)).Exist(ctx)
			if err != nil {
				log.Printf("db check %s: %v", obj.Key, err)
				continue
			}
			if exists {
				continue
			}

			info, err := m.StatObject(ctx, m.Bucket, obj.Key, mc.StatObjectOptions{})
			if err != nil {
				log.Printf("stat object %s: %v", obj.Key, err)
				continue
			}

			ingest.Process(ctx, cfg, m, db, obj.Key, info.ContentType)
		}

		c.Status(http.StatusOK)
	}
}

func exportTagsHandler(db *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		items, err := db.Media.Query().Where(media.HasTags()).WithTags().All(ctx)
		if err != nil {
			log.Printf("export tags: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.Header("Content-Type", "application/x-ndjson")
		c.Header("Content-Encoding", "gzip")
		c.Header("Content-Disposition", "attachment; filename=\"tags_export.ndjson.gz\"")

		gz := gzip.NewWriter(c.Writer)
		defer gz.Close()

		enc := json.NewEncoder(gz)
		meta := struct {
			Version   int       `json:"version"`
			CreatedAt time.Time `json:"createdAt"`
		}{Version: 1, CreatedAt: time.Now().UTC()}
		if err := enc.Encode(meta); err != nil {
			log.Printf("encode meta: %v", err)
			return
		}

		for _, m := range items {
			if len(m.Edges.Tags) == 0 {
				continue
			}
			tags := make([]string, len(m.Edges.Tags))
			for i, t := range m.Edges.Tags {
				tags[i] = t.Name
			}
			if err := enc.Encode(struct {
				ID   string   `json:"id"`
				Tags []string `json:"tags"`
			}{ID: m.ID, Tags: tags}); err != nil {
				log.Printf("encode record %s: %v", m.ID, err)
				return
			}
		}
	}
}
