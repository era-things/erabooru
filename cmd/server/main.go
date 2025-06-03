package main

import (
	"era/booru/internal/assets"
	"net/http"
	"strings"
	"embed"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Create a file server from the embedded FS 
	fs := http.FS(assets.UI)

	// Serve static files from the embedded build directory
	r.GET("/static/*filepath", func(c *gin.Context) {
		path := "build" + c.Request.URL.Path
		// Prevent directory traversal
		if strings.Contains(c.Param("filepath"), "..") {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.FileFromFS(path, fs)
	})

	// Serve index.html for root
	r.GET("/", serveIndex(assets.UI))

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