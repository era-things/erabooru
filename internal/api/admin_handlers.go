package api

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"era/booru/ent"
	"era/booru/ent/media"
	"era/booru/ent/tag"
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
	r.POST("/api/admin/import-tags", importTagsHandler(db))
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

		c.Header("Content-Type", "application/gzip")
		c.Header("Content-Disposition", "attachment; filename=\"tags_export.ndjson.gz\"")

		gz := gzip.NewWriter(c.Writer)
		defer gz.Close()

		enc := json.NewEncoder(gz)
		meta := struct {
			Version   int       `json:"version"`
			CreatedAt time.Time `json:"createdAt"`
		}{Version: 2, CreatedAt: time.Now().UTC()}
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
				ID         string   `json:"id"`
				Tags       []string `json:"tags"`
				UploadDate string   `json:"upload_date"`
			}{ID: m.ID, Tags: tags, UploadDate: m.UploadDate.Format("2006-01-02")}); err != nil {
				log.Printf("encode record %s: %v", m.ID, err)
				return
			}
		}
	}
}

func importTagsHandler(db *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		gz, err := gzip.NewReader(c.Request.Body)
		if err != nil {
			log.Printf("gzip reader error: %v", err)
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		defer gz.Close()

		dec := json.NewDecoder(gz)

		var meta struct {
			Version   int       `json:"version"`
			CreatedAt time.Time `json:"createdAt"`
		}
		if err := dec.Decode(&meta); err != nil {
			log.Printf("decode meta: %v", err)
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		for {
			var item struct {
				ID         string   `json:"id"`
				Tags       []string `json:"tags"`
				UploadDate string   `json:"upload_date"`
			}
			if err := dec.Decode(&item); err != nil {
				if err == io.EOF {
					break
				}
				log.Printf("decode item: %v", err)
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}

			if len(item.Tags) == 0 && item.UploadDate == "" {
				continue
			}

			mobj, err := db.Media.Query().Where(media.IDEQ(item.ID)).WithTags().Only(ctx)
			if ent.IsNotFound(err) {
				continue
			}
			if err != nil {
				log.Printf("query media %s: %v", item.ID, err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			// Parse import date
			var importDate *time.Time
			if item.UploadDate != "" {
				if t, err := time.Parse("2006-01-02", item.UploadDate); err == nil {
					importDate = &t
				}
			}

			// Compare dates - only proceed if import date is newer or equal
			if importDate != nil {
				currentDate := mobj.UploadDate
				// Check if currentDate is not nil before dereferencing
				if currentDate != nil && importDate.Before(*currentDate) {
					log.Printf("skipping media %s: import date %s is earlier than current date %s",
						item.ID, importDate.Format("2006-01-02"), currentDate.Format("2006-01-02"))
					continue
				}
			}

			// Build map of existing tags (only if we have tags to process)
			var toAdd []int
			if len(item.Tags) > 0 {
				existing := make(map[string]struct{}, len(mobj.Edges.Tags))
				for _, t := range mobj.Edges.Tags {
					existing[t.Name] = struct{}{}
				}

				// Find new tags to add
				for _, name := range normalizeTags(item.Tags) {
					if _, ok := existing[name]; ok {
						continue
					}
					tg, err := db.Tag.Query().Where(tag.NameEQ(name)).Only(ctx)
					if ent.IsNotFound(err) {
						tg, err = db.Tag.Create().SetName(name).SetType(tag.TypeUserTag).Save(ctx)
					}
					if err != nil {
						log.Printf("lookup tag %s: %v", name, err)
						c.AbortWithStatus(http.StatusInternalServerError)
						return
					}
					existing[name] = struct{}{}
					toAdd = append(toAdd, tg.ID)
				}
			}

			// Update media with new tags and/or date
			upd := db.Media.UpdateOneID(item.ID)
			if len(toAdd) > 0 {
				upd = upd.AddTagIDs(toAdd...)
			}
			if importDate != nil {
				upd = upd.SetUploadDate(*importDate)
			}

			// Only save if we have something to update
			if len(toAdd) > 0 || importDate != nil {
				if _, err := upd.Save(ctx); err != nil {
					log.Printf("update media %s: %v", item.ID, err)
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}

				log.Printf("updated media %s: added %d tags, set date to %s",
					item.ID, len(toAdd), item.UploadDate)
			}
		}

		c.Status(http.StatusOK)
	}
}
