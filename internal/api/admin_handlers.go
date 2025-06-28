package api

import (
	"log"
	"net/http"

	"era/booru/ent"
	"era/booru/internal/config"
	"era/booru/internal/search"

	"github.com/gin-gonic/gin"
)

// RegisterAdminRoutes registers admin-only endpoints.
func RegisterAdminRoutes(r *gin.Engine, db *ent.Client, cfg *config.Config) {
	r.POST("/api/admin/regenerate", regenerateHandler(db, cfg))
}

func regenerateHandler(db *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := search.Rebuild(c.Request.Context(), db, cfg.BlevePath); err != nil {
			log.Printf("regenerate index: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}
