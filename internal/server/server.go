package server

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"era/booru/ent"
	"era/booru/internal/api"
	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/ingest"
	"era/booru/internal/minio"
	"era/booru/internal/search"
)

// Server bundles initialized services and the Gin router.
type Server struct {
	Router *gin.Engine
	DB     *ent.Client
	Minio  *minio.Client
	Cfg    *config.Config
	ctx    context.Context
	cancel context.CancelFunc
}

// New constructs a Server and starts the MinIO watcher.
func New(ctx context.Context, cfg *config.Config) (*Server, error) {
	if err := search.OpenOrCreate(cfg.BlevePath); err != nil {
		return nil, err
	}

	m, err := minio.New(cfg)
	if err != nil {
		search.Close()
		return nil, err
	}

	database, err := db.New(cfg)
	if err != nil {
		search.Close()
		return nil, err
	}

	srvCtx, cancel := context.WithCancel(ctx)

	go m.Watch(srvCtx, func(ctx context.Context, object, contentType string) {
		mediaID, err := ingest.Process(ctx, cfg, m, database, object, contentType)
		if err != nil {
			log.Printf("process %s: %v", object, err)
			return
		}
		if mediaID == "" {
			return
		}
		tagme, err := db.FindOrCreateTag(ctx, database, "tagme")
		if err != nil {
			log.Printf("tagme lookup: %v", err)
			return
		}
		if _, err := database.Media.UpdateOneID(mediaID).AddTagIDs(tagme.ID).Save(ctx); err != nil {
			log.Printf("add tagme: %v", err)
		}
		log.Printf("processed %s (%s)", object, mediaID)
	})

	r := gin.New()
	r.Use(api.GinLogger(), gin.Recovery(), api.CORSMiddleware())
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	api.RegisterMediaRoutes(r, database, m, cfg)
	api.RegisterAdminRoutes(r, database, m, cfg)
	api.RegisterStaticRoutes(r)

	s := &Server{
		Router: r,
		DB:     database,
		Minio:  m,
		Cfg:    cfg,
		ctx:    srvCtx,
		cancel: cancel,
	}
	return s, nil
}

// Run starts serving using Gin's Run.
func (s *Server) Run(addr string) error {
	return s.Router.Run(addr)
}

// Close shuts down background resources.
func (s *Server) Close() {
	s.cancel()
	s.DB.Close()
	search.Close()
}
