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
	"era/booru/ent/predicate"
	"era/booru/internal/config"
	"era/booru/internal/minio"

	"entgo.io/ent/dialect/sql"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	mc "github.com/minio/minio-go/v7"
)

// parseSearch converts a simple query string like "width>100 height<=200 type=image"
// into a slice of ent predicates used for filtering Media.
func parseSearch(q string) []predicate.Media {
	tokens := strings.Fields(q)
	preds := make([]predicate.Media, 0, len(tokens))
	for _, t := range tokens {
		var field, op, val string
		for _, o := range []string{">=", "<=", ">", "<", "="} {
			if idx := strings.Index(t, o); idx > 0 {
				field = t[:idx]
				op = o
				val = t[idx+len(o):]
				break
			}
		}
		if field == "" {
			continue
		}

		if v, err := strconv.Atoi(val); err == nil {
			switch op {
			case "=":
				preds = append(preds, predicate.Media(sql.FieldEQ(field, v)))
			case ">":
				preds = append(preds, predicate.Media(sql.FieldGT(field, v)))
			case "<":
				preds = append(preds, predicate.Media(sql.FieldLT(field, v)))
			case ">=":
				preds = append(preds, predicate.Media(sql.FieldGTE(field, v)))
			case "<=":
				preds = append(preds, predicate.Media(sql.FieldLTE(field, v)))
			}
		} else {
			if op == "=" {
				preds = append(preds, predicate.Media(sql.FieldEQ(field, val)))
			}
		}
	}
	return preds
}

func RegisterMediaRoutes(ginEngine *gin.Engine, database *ent.Client, minioClient *minio.Client, cfg *config.Config) {
	ginEngine.GET("/api/media", func(c *gin.Context) {
		q := c.Query("q")
		query := database.Media.Query()
		preds := parseSearch(q)
		if len(preds) > 0 {
			query = query.Where(preds...)
		}

		items, err := query.
			Limit(50).
			Order(media.ByID(sql.OrderDesc())).
			All(c.Request.Context())
		if err != nil {
			log.Printf("query media: %v", err)
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

		item, err := database.Media.Get(c.Request.Context(), id)
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
		c.JSON(http.StatusOK, gin.H{
			"id":     item.ID,
			"url":    url,
			"width":  item.Width,
			"height": item.Height,
			"format": item.Format,
			"size":   stat.Size,
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
