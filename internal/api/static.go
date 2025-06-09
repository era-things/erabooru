package api

import (
	"embed"
	"net/http"
	"strings"

	"era/booru/internal/assets"

	"github.com/gin-gonic/gin"
)

func RegisterStaticRoutes(r *gin.Engine) {
	// Serve assets from the embedded build directory
	r.GET("/_app/*filepath", serveStatic)
	r.GET("/favicon.png", serveStatic)

	// SPA fallback - serve index.html for all other routes
	r.NoRoute(serveIndex(assets.UI))
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
