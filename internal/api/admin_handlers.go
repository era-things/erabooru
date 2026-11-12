package api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"era/booru/internal/db"
	embed "era/booru/internal/embeddings"
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
		err := requeueCleanup(ctx, pool, riverClient)
		if err == nil {
			err = store.WalkOriginals(func(id string) error {
				tx, err := pool.Begin(ctx)
				if err != nil {
					return err
				}
				defer tx.Rollback(context.WithoutCancel(ctx))
				if err := db.LockMediaTx(ctx, tx, id); err != nil {
					return err
				}
				deleted, err := db.IsDeletedTx(ctx, tx, id)
				if err != nil {
					return err
				}
				if deleted {
					return nil
				}
				file, err := os.Open(store.OriginalPath(id))
				if os.IsNotExist(err) {
					return nil
				}
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
				generation, status, lifecycleErr := db.MediaLifecycle(ctx, tx, id)
				if lifecycleErr == nil && status == "ready" {
					if !embed.Available() {
						return nil
					}
					embeddingStatus, embeddingErr := db.EmbeddingLifecycle(ctx, tx, id, db.DefaultEmbeddingSpace)
					if embeddingErr == nil && embeddingStatus == "ready" {
						return nil
					}
					active, err := queue.HasActiveJobTx(ctx, tx, queue.EmbedArgs{Key: id, Generation: generation, Space: db.DefaultEmbeddingSpace})
					if err != nil {
						return err
					}
					if active {
						return nil
					}
					if embeddingErr != nil && !errors.Is(embeddingErr, pgx.ErrNoRows) {
						return fmt.Errorf("check embedding %s: %w", id, embeddingErr)
					}
					queued, err := db.QueueEmbeddingTx(ctx, tx, id, generation, db.DefaultEmbeddingSpace)
					if err != nil {
						return err
					}
					if !queued {
						return tx.Commit(ctx)
					}
					if err := queue.EnqueueTx(ctx, riverClient, tx, queue.EmbedArgs{Key: id, Generation: generation, Space: db.DefaultEmbeddingSpace}); err != nil {
						return err
					}
					if err := tx.Commit(ctx); err != nil {
						return err
					}
					embedCount++
					return nil
				}
				if lifecycleErr != nil && !errors.Is(lifecycleErr, pgx.ErrNoRows) {
					return fmt.Errorf("check media %s: %w", id, lifecycleErr)
				}
				if lifecycleErr == nil {
					active, err := queue.HasActiveJobTx(ctx, tx, queue.ProcessArgs{Key: id, Generation: generation})
					if err != nil {
						return err
					}
					if active {
						return nil
					}
				}
				if errors.Is(lifecycleErr, pgx.ErrNoRows) {
					generation, err = newGeneration()
					if err != nil {
						return err
					}
				}
				stat, err := os.Stat(store.OriginalPath(id))
				if err != nil {
					return err
				}
				if errors.Is(lifecycleErr, pgx.ErrNoRows) {
					if _, err := db.InsertQueuedMedia(ctx, tx, id, generation, "", stat.Size(), time.Now().UTC()); err != nil {
						return err
					}
				} else if err := db.RequeueIngestionTx(ctx, tx, id, generation); err != nil {
					return err
				}
				if err := queue.EnqueueTx(ctx, riverClient, tx, queue.ProcessArgs{Key: id, Generation: generation, ContentType: contentType}); err != nil {
					return fmt.Errorf("enqueue media %s: %w", id, err)
				}
				if err := tx.Commit(ctx); err != nil {
					return err
				}

				processCount++
				return nil
			})
		}
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
		items, err := db.ListMediaMetadata(ctx, pool)
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
		}{Version: 3, CreatedAt: time.Now().UTC()}
		if err := enc.Encode(meta); err != nil {
			log.Printf("encode meta: %v", err)
			return
		}

		for _, media := range items {
			item := metadataRecord{ID: media.ID, OriginalName: media.OriginalName, UploadedAt: media.UploadedAt, Tags: media.Tags,
				NegativeTags: media.NegativeTags, Dates: make([]metadataDate, len(media.Dates))}
			for i, date := range media.Dates {
				item.Dates[i] = metadataDate{Name: date.Name, Value: date.Value.Format("2006-01-02")}
			}
			if err := enc.Encode(item); err != nil {
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

		if meta.Version < 1 || meta.Version > 3 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported metadata version"})
			return
		}
		for {
			var item metadataRecord
			if err := dec.Decode(&item); err != nil {
				if err == io.EOF {
					break
				}
				log.Printf("decode item: %v", err)
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			metadata, err := item.metadata(meta.Version)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if err := db.RestoreMediaMetadata(ctx, pool, metadata); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					continue
				}
				log.Printf("import metadata for %s: %v", item.ID, err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
		}

		c.Status(http.StatusOK)
	}
}

// Exhausted cleanup jobs can be retried by the repair action as well.
func requeueCleanup(ctx context.Context, pool *pgxpool.Pool, client *river.Client[pgx.Tx]) error {
	rows, err := pool.Query(ctx, `SELECT id, generation FROM deleted_media WHERE cleaned_at IS NULL`)
	if err != nil {
		return err
	}
	var jobs []queue.CleanupArgs
	for rows.Next() {
		var job queue.CleanupArgs
		if err := rows.Scan(&job.Key, &job.Generation); err != nil {
			rows.Close()
			return err
		}
		jobs = append(jobs, job)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, job := range jobs {
		if err := queue.Enqueue(ctx, client, job); err != nil {
			return err
		}
	}
	return nil
}
