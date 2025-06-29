package api

import (
	"era/booru/ent"
	"era/booru/internal/config"
	"era/booru/internal/minio"
	"github.com/gin-gonic/gin"
	"net/http"
)

// GoaServer implements api.ServerInterface by delegating to existing handlers.
type GoaServer struct {
	adminRegenerate gin.HandlerFunc
	mediaList       gin.HandlerFunc
	mediaPreviews   gin.HandlerFunc
	mediaGet        gin.HandlerFunc
	mediaUploadURL  gin.HandlerFunc
	mediaUpdateTags gin.HandlerFunc
	mediaDelete     gin.HandlerFunc
	healthCheck     gin.HandlerFunc
}

func NewGoaServer(db *ent.Client, m *minio.Client, cfg *config.Config) *GoaServer {
	return &GoaServer{
		adminRegenerate: regenerateHandler(db, m, cfg),
		mediaList:       listMediaHandler(cfg),
		mediaPreviews:   listPreviewsHandler(cfg),
		mediaGet:        getMediaHandler(db, m, cfg),
		mediaUploadURL:  uploadURLHandler(m, cfg),
		mediaUpdateTags: updateMediaTagsHandler(db),
		mediaDelete:     deleteMediaHandler(db, m),
		healthCheck: func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		},
	}
}

func (s *GoaServer) AdminRegenerate(c *gin.Context) {
	s.adminRegenerate(c)
}

func (s *GoaServer) MediaList(c *gin.Context, params MediaListParams) {
	s.mediaList(c)
}

func (s *GoaServer) MediaPreviews(c *gin.Context, params MediaPreviewsParams) {
	s.mediaPreviews(c)
}

func (s *GoaServer) MediaUploadURL(c *gin.Context) {
	s.mediaUploadURL(c)
}

func (s *GoaServer) MediaDelete(c *gin.Context, id string) {
	// id already in path, handler reads from context
	s.mediaDelete(c)
}

func (s *GoaServer) MediaGet(c *gin.Context, id string) {
	s.mediaGet(c)
}

func (s *GoaServer) MediaUpdateTags(c *gin.Context, id string) {
	s.mediaUpdateTags(c)
}

func (s *GoaServer) HealthCheck(c *gin.Context) {
	s.healthCheck(c)
}
