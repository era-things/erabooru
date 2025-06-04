package main

import (
	"embed"
	"era/booru/internal/assets"
	"era/booru/internal/config"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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
	_ = cfg

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), corsMiddleware())

	// Serve assets from the embedded build directory
	r.GET("/_app/*filepath", serveStatic)
	r.GET("/favicon.png", serveStatic)

	// SPA fallback - serve index.html for all other routes
	r.NoRoute(serveIndex(assets.UI))

	r.Run(":8080")
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
