package main

import (
	"embed"
	"era/booru/internal/assets"
	"era/booru/internal/config"
	minio "era/booru/internal/minio"
	"fmt"
	mc "github.com/minio/minio-go/v7"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"era/booru/ent/media"

	"entgo.io/ent/dialect/sql"

	"context"
	"era/booru/internal/db"
	"os/signal"
	"syscall"
)

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept")
		c.Header("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("error loading configuration: %v", err)
	}

	m, err := minio.New(cfg)
	if err != nil {
		log.Fatalf("init minio: %v", err)
	}

	database, err := db.New(cfg)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("watching for new uploads")
	go m.Watch(ctx, database)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), corsMiddleware())

	r.POST("/api/upload-url", func(c *gin.Context) {
		type req struct {
			Filename string `json:"filename"`
		}
		var body req
		if err := c.BindJSON(&body); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		if !strings.HasSuffix(strings.ToLower(body.Filename), ".png") {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		object := uuid.New().String() + ".png"
		url, err := m.PresignedPut(c.Request.Context(), cfg, object, time.Minute*15)
		if err != nil {
			log.Printf("presign: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, gin.H{"url": url, "object": object})
	})

	r.GET("/api/media", func(c *gin.Context) {
		items, err := database.Media.Query().
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

	r.GET("/api/media/:id", func(c *gin.Context) {
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

		stat, err := m.StatObject(c.Request.Context(), m.Bucket, item.Key, mc.StatObjectOptions{})
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

	r.DELETE("/api/media/:id", func(c *gin.Context) {
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

		if err := m.RemoveObject(c.Request.Context(), m.Bucket, item.Key, mc.RemoveObjectOptions{}); err != nil {
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

	// Serve assets from the embedded build directory
	r.GET("/_app/*filepath", serveStatic)
	r.GET("/favicon.png", serveStatic)

	// SPA fallback - serve index.html for all other routes
	r.NoRoute(serveIndex(assets.UI))

	r.Run(":8080")
	log.Printf("Server running on http://localhost:8080")

}

func serveIndex(ui embed.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, err := ui.ReadFile("build/index.html")
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html", file)
	}
}

func serveStatic(c *gin.Context) {
	path := "build" + c.Request.URL.Path

	// Prevent directory traversal
	if strings.Contains(c.Param("filepath"), "..") {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	fs := http.FS(assets.UI)

	// Check if file exists
	f, err := fs.Open(path)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	f.Close()

	c.FileFromFS(path, fs)
}
