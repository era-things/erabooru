package api

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"era/booru/internal/db"
	"era/booru/internal/queue"
	"era/booru/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// RegisterAdminRoutes registers admin-only endpoints.
func RegisterAdminRoutes(r *gin.Engine, pool *pgxpool.Pool, store *storage.Store, riverClient *river.Client[pgx.Tx]) {
	r.POST("/api/admin/regenerate", regenerateHandler(pool, store, riverClient))
	r.GET("/api/admin/export-tags", exportTagsHandler(pool))
	r.POST("/api/admin/import-tags", importTagsHandler(pool))
}

func regenerateHandler(pool *pgxpool.Pool, store *storage.Store, riverClient *river.Client[pgx.Tx]) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		processCount := 0
		embedCount := 0
		err := store.WalkOriginals(func(id string) error {
			exists, err := db.MediaExists(ctx, pool, id)
			if err != nil {
				return fmt.Errorf("check media %s: %w", id, err)
			}
			if exists {
				hasVector, err := db.MediaVectorExists(ctx, pool, id, "vision")
				if err != nil {
					return fmt.Errorf("check embedding %s: %w", id, err)
				}
				if !hasVector {
					if err := queue.Enqueue(ctx, riverClient, queue.EmbedArgs{Key: id}); err != nil {
						return fmt.Errorf("enqueue embedding %s: %w", id, err)
					}
					embedCount++
				}
				return nil
			}

			file, err := os.Open(store.OriginalPath(id))
			if err != nil {
				return err
			}
			var header [512]byte
			n, readErr := file.Read(header[:])
			_ = file.Close()
			if readErr != nil && readErr != io.EOF {
				return readErr
			}
			contentType := http.DetectContentType(header[:n])

			_, err = riverClient.Insert(ctx, queue.ProcessArgs{
				Key:         id,
				ContentType: contentType,
			}, &river.InsertOpts{
				Queue: "process",
			})
			if err != nil {
				return fmt.Errorf("enqueue media %s: %w", id, err)
			}

			processCount++
			return nil
		})
		if err != nil {
			log.Printf("repair scan: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		log.Printf("repair scan completed, enqueued %d processing and %d embedding jobs", processCount, embedCount)
		c.JSON(http.StatusOK, gin.H{
			"message":         "repair scan completed",
			"jobs_enqueued":   processCount + embedCount,
			"processing_jobs": processCount,
			"embedding_jobs":  embedCount,
		})
	}
}

func exportTagsHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		items, err := db.ListMedia(ctx, pool)
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

		for _, media := range items {
			tags := make([]string, len(media.Tags))
			for i, tag := range media.Tags {
				tags[i] = tag.Name
			}

			dates := make([]struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			}, 0)
			for _, date := range media.Dates {
				dates = append(dates, struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				}{Name: date.Name, Value: date.Value.Format("2006-01-02")})
			}

			if err := enc.Encode(struct {
				ID    string   `json:"id"`
				Tags  []string `json:"tags"`
				Dates []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"dates"`
			}{ID: media.ID, Tags: tags, Dates: dates}); err != nil {
				log.Printf("encode record %s: %v", media.ID, err)
				return
			}
		}
	}
}

func importTagsHandler(pool *pgxpool.Pool) gin.HandlerFunc {
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
				ID    string   `json:"id"`
				Tags  []string `json:"tags"`
				Dates []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"dates"`
			}
			if err := dec.Decode(&item); err != nil {
				if err == io.EOF {
					break
				}
				log.Printf("decode item: %v", err)
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}

			exists, err := db.MediaExists(ctx, pool, item.ID)
			if err != nil {
				log.Printf("check media %s: %v", item.ID, err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			if !exists {
				continue
			}
			dates := make([]db.DateValue, 0, len(item.Dates))
			for _, d := range item.Dates {
				value, err := time.Parse("2006-01-02", d.Value)
				if err != nil {
					continue
				}
				dates = append(dates, db.DateValue{Name: d.Name, Value: value})
			}
			if err := db.SetMediaTags(ctx, pool, item.ID, normalizeTags(item.Tags)); err != nil {
				log.Printf("import tags for %s: %v", item.ID, err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			if err := db.SetMediaDates(ctx, pool, item.ID, dates); err != nil {
				log.Printf("import dates for %s: %v", item.ID, err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
		}

		c.Status(http.StatusOK)
	}
}
