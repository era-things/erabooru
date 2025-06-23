package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"era/booru/ent"
	"era/booru/ent/media"
	"era/booru/ent/tag"
	"era/booru/internal/config"
	"era/booru/internal/minio"
	"era/booru/internal/search"

	"github.com/gin-gonic/gin"
	mc "github.com/minio/minio-go/v7"
)

func RegisterMediaRoutes(r *gin.Engine, db *ent.Client, m *minio.Client, cfg *config.Config) {
	r.GET("/api/media", listMediaHandler(cfg))
	r.GET("/api/media/previews", listPreviewsHandler(cfg))
	r.GET("/api/media/:id", getMediaHandler(db, m, cfg))
	r.POST("/api/media/upload-url", uploadURLHandler(m, cfg))
	r.POST("/api/media/:id/tags", updateMediaTagsHandler(db))
	r.DELETE("/api/media/:id", deleteMediaHandler(db, m))
}

func listMediaHandler(cfg *config.Config) gin.HandlerFunc {
	return listCommon(cfg.MinioBucket, cfg.MinioBucket)
}

func listPreviewsHandler(cfg *config.Config) gin.HandlerFunc {
	//for now, image previews are just original full-size images
	return listCommon(cfg.PreviewBucket, cfg.MinioBucket)
}

func listCommon(videoBucket string, pictureBucket string) gin.HandlerFunc {
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

			url := fmt.Sprintf("http://localhost/minio/%s/%s", bucket, key)
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
			return
		}

		item, err := db.Media.Query().Where(media.IDEQ(id)).WithTags().Only(c.Request.Context())
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

		url := fmt.Sprintf("http://localhost/minio/%s/%s", cfg.MinioBucket, string(id))
		tags := make([]string, len(item.Edges.Tags))
		for i, t := range item.Edges.Tags {
			tags[i] = t.Name
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

func updateMediaTagsHandler(db *ent.Client) gin.HandlerFunc {
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

		tagIDs := make([]int, 0, len(clean))
		for _, name := range clean {
			tg, err := db.Tag.Query().Where(tag.NameEQ(name)).Only(c.Request.Context())
			if ent.IsNotFound(err) {
				tg, err = db.Tag.Create().SetName(name).SetType(tag.TypeUserTag).Save(c.Request.Context())
			}
			if err != nil {
				log.Printf("tag lookup/create %s: %v", name, err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			tagIDs = append(tagIDs, tg.ID)
		}

		if _, err := db.Media.UpdateOneID(id).ClearTags().AddTagIDs(tagIDs...).Save(c.Request.Context()); err != nil {
			log.Printf("update media tags %s: %v", id, err)
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
