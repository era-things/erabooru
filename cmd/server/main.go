package main

import (
        "era/booru/internal/assets"
        "net/http"

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
        r := gin.New()
        r.Use(gin.Logger(), gin.Recovery(), corsMiddleware())

	// --- API ---
	//api := r.Group("/api")
	//api.GET("/search", apiSearch) // handlers in internal/api
	// … other endpoints …

	// --- Static UI (embedded) ---
	r.StaticFS("/", http.FS(assets.UI)) // serves JS/CSS/img  :contentReference[oaicite:2]{index=2}
	r.NoRoute(func(c *gin.Context) {    // SPA client routes
		c.FileFromFS("web/build/index.html", http.FS(assets.UI))
	})

	r.Run(":8080")
}
