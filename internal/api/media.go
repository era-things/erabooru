package api

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"era/booru/ent"
	"era/booru/ent/media"
	"era/booru/ent/tag"
	"era/booru/internal/config"
	"era/booru/internal/minio"
	"era/booru/internal/search"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	mc "github.com/minio/minio-go/v7"
)

func RegisterMediaRoutes(ginEngine *gin.Engine, database *ent.Client, minioClient *minio.Client, cfg *config.Config) {
	ginEngine.GET("/api/media", func(c *gin.Context) {
		q := c.Query("q")
		items, err := search.SearchMedia(q, 50)
		if err != nil {
			log.Printf("search media: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		out := make([]gin.H, len(items))
		for i, mitem := range items {
			url := fmt.Sprintf("http://localhost/minio/%s/%s", cfg.MinioBucket, mitem.Key)
			out[i] = gin.H{
				"id":     mitem.ID,
				"url":    url,
				"width":  mitem.Width,
				"height": mitem.Height,
			}
		}

		c.JSON(http.StatusOK, gin.H{"media": out})
	})

	ginEngine.GET("/api/media/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		item, err := database.Media.Query().Where(media.IDEQ(id)).WithTags().Only(c.Request.Context())
		if err != nil {
			log.Printf("get media %d: %v", id, err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		stat, err := minioClient.StatObject(c.Request.Context(), minioClient.Bucket, item.Key, mc.StatObjectOptions{})
		if err != nil {
			log.Printf("stat object %s: %v", item.Key, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		url := fmt.Sprintf("http://localhost/minio/%s/%s", cfg.MinioBucket, item.Key)
		tags := make([]string, len(item.Edges.Tags))
		for i, t := range item.Edges.Tags {
			tags[i] = t.Name
		}
		c.JSON(http.StatusOK, gin.H{
			"id":     item.ID,
			"url":    url,
			"width":  item.Width,
			"height": item.Height,
			"format": item.Format,
			"size":   stat.Size,
			"tags":   tags,
		})
	})

	ginEngine.POST("/api/media/upload-url", func(c *gin.Context) {
		type req struct {
			Filename string `json:"filename"`
		}
		var body req
		if err := c.BindJSON(&body); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		object, err := CreateMediaName(body.Filename)
		if err != nil {
			log.Printf("create media name: %v", err)
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		url, err := minioClient.PresignedPut(c.Request.Context(), cfg, object, time.Minute*15)
		if err != nil {
			log.Printf("presign: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, gin.H{"url": url, "object": object})
	})

	ginEngine.POST("/api/media/:id/tags", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		var body struct {
			Tags []string `json:"tags"`
		}
		if err := c.BindJSON(&body); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		// Normalize tags: underscores to spaces, trim, deduplicate
		seen := map[string]struct{}{}
		clean := make([]string, 0, len(body.Tags))
		for _, t := range body.Tags {
			t = strings.TrimSpace(t)
			log.Printf("tag: %s", t)
			if t == "" {
				continue
			}
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				clean = append(clean, t)
			}
		}

		tagIDs := make([]int, 0, len(clean))
		for _, name := range clean {
			tg, err := database.Tag.Query().Where(tag.NameEQ(name)).Only(c.Request.Context())
			if ent.IsNotFound(err) {
				tg, err = database.Tag.Create().SetName(name).SetType(tag.TypeUserTag).Save(c.Request.Context())
			}
			if err != nil {
				log.Printf("tag lookup/create %s: %v", name, err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			tagIDs = append(tagIDs, tg.ID)
		}

		if _, err := database.Media.UpdateOneID(id).ClearTags().AddTagIDs(tagIDs...).Save(c.Request.Context()); err != nil {
			log.Printf("update media tags %d: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.Status(http.StatusOK)
	})

	ginEngine.DELETE("/api/media/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		item, err := database.Media.Get(c.Request.Context(), id)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		if err := minioClient.RemoveObject(c.Request.Context(), minioClient.Bucket, item.Key, mc.RemoveObjectOptions{}); err != nil {
			log.Printf("remove object %s: %v", item.Key, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		if err := database.Media.DeleteOneID(id).Exec(c.Request.Context()); err != nil {
			log.Printf("delete media %d: %v", id, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.Status(http.StatusOK)
	})
}

func CreateMediaName(filename string) (string, error) {
	extension := filepath.Ext(filename)
	// Validate file extension
	switch strings.ToLower(extension) {
	case ".png", ".jpg", ".jpeg", ".gif":
		// Valid formats
	default:
		return "", fmt.Errorf("unsupported file format: %s", extension)
	}

	// Generate a unique name using UUID
	name := uuid.New().String() + extension
	return name, nil
}
