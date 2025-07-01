package api

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"era/booru/ent"
	"era/booru/ent/attribute"
	"era/booru/ent/media"
	"era/booru/internal/config"
	dbhelpers "era/booru/internal/db"
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
		// Remove the HasTags() filter to export all media
		items, err := db.Media.Query().WithTags().All(ctx)
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
			// Remove the check for len(m.Edges.Tags) == 0 to export all media
			tags := make([]string, len(m.Edges.Tags))
			for i, t := range m.Edges.Tags {
				tags[i] = t.Name
			}

			props, err := dbhelpers.ListProperties(ctx, db, m.ID)
			if err != nil {
				log.Printf("list properties %s: %v", m.ID, err)
				return
			}
			outProps := make([]struct {
				Name  string `json:"name"`
				Type  string `json:"type"`
				Value string `json:"value"`
			}, 0, len(props))
			for _, p := range props {
				if p.Value == nil {
					continue
				}
				outProps = append(outProps, struct {
					Name  string `json:"name"`
					Type  string `json:"type"`
					Value string `json:"value"`
				}{Name: p.Name, Type: string(p.Type), Value: *p.Value})
			}

			if err := enc.Encode(struct {
				ID         string   `json:"id"`
				Tags       []string `json:"tags"`
				Properties []struct {
					Name  string `json:"name"`
					Type  string `json:"type"`
					Value string `json:"value"`
				} `json:"properties"`
			}{ID: m.ID, Tags: tags, Properties: outProps}); err != nil {
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
				Properties []struct {
					Name  string `json:"name"`
					Type  string `json:"type"`
					Value string `json:"value"`
				} `json:"properties"`
			}
			if err := dec.Decode(&item); err != nil {
				if err == io.EOF {
					break
				}
				log.Printf("decode item: %v", err)
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}

			// Don't skip - process every record to potentially remove "tagme"
			mobj, err := db.Media.Query().Where(media.IDEQ(item.ID)).WithTags().Only(ctx)
			if ent.IsNotFound(err) {
				continue // Skip if media doesn't exist
			}
			if err != nil {
				log.Printf("query media %s: %v", item.ID, err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			// 1) Always look for existing "tagme" tag and remove it if it exists
			var tagmeID *int
			for _, t := range mobj.Edges.Tags {
				if t.Name == "tagme" {
					tagmeID = &t.ID
					break
				}
			}

			// 2) Handle imported properties generically
			var propertyChanges []string
			for _, p := range item.Properties {
				at, err := dbhelpers.FindOrCreateProperty(ctx, db, p.Name, attribute.Type(p.Type))
				if err != nil {
					log.Printf("lookup property %s: %v", p.Name, err)
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}
				switch attribute.Type(p.Type) {
				case attribute.TypeDate:
					t, err := time.Parse("2006-01-02", p.Value)
					if err != nil {
						continue
					}
					if err := dbhelpers.SetDateProperty(ctx, db, item.ID, at.ID, t); err != nil {
						log.Printf("set date property %s: %v", p.Name, err)
						c.AbortWithStatus(http.StatusInternalServerError)
						return
					}
					propertyChanges = append(propertyChanges, fmt.Sprintf("set %s", p.Name))
				}
			}

			// 3) Parse existing tags and merge with incoming tags
			var toAdd []int
			if len(item.Tags) > 0 {
				existing := make(map[string]struct{}, len(mobj.Edges.Tags))
				for _, t := range mobj.Edges.Tags {
					existing[t.Name] = struct{}{}
				}

				// Find new tags to add
				for _, name := range normalizeTags(item.Tags) {
					if _, ok := existing[name]; ok {
						continue // Tag already exists
					}
					tg, err := db.Attribute.Query().Where(attribute.NameEQ(name)).Only(ctx)
					if ent.IsNotFound(err) {
						tg, err = db.Attribute.Create().SetName(name).SetType(attribute.TypeTag).Save(ctx)
					}
					if err != nil {
						log.Printf("lookup tag %s: %v", name, err)
						c.AbortWithStatus(http.StatusInternalServerError)
						return
					}
					toAdd = append(toAdd, tg.ID)
				}
			}

			// Update media with changes
			upd := db.Media.UpdateOneID(item.ID)
			var changes []string

			// Always remove "tagme" tag if it exists
			if tagmeID != nil {
				upd = upd.RemoveTagIDs(*tagmeID)
				changes = append(changes, "removed 'tagme' tag")
			}

			// Add new tags
			if len(toAdd) > 0 {
				upd = upd.AddTagIDs(toAdd...)
				changes = append(changes, fmt.Sprintf("added %d tags", len(toAdd)))
			}

			changes = append(changes, propertyChanges...)

			if len(changes) > 0 {
				if _, err := upd.Save(ctx); err != nil {
					log.Printf("update media %s: %v", item.ID, err)
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}
				log.Printf("updated media %s: %s", item.ID, strings.Join(changes, ", "))
			}
		}

		c.Status(http.StatusOK)
	}
}
