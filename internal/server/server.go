package server

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"era/booru/ent"
	"era/booru/internal/api"
	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/minio"
	"era/booru/internal/queue"
	qworkers "era/booru/internal/queue/workers"
	"era/booru/internal/search"
	"github.com/riverqueue/river"
)

// Server bundles initialized services and the Gin router.
type Server struct {
	Router *gin.Engine
	DB     *ent.Client
	Minio  *minio.Client
	Cfg    *config.Config
	Queue  *river.Client[*sql.Tx]
	DBPool *sql.DB
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
	workers := river.NewWorkers()
	dbpool, err := sql.Open("postgres", cfg.PostgresDSN)
	if err != nil {
		return nil, err
	}
	riverClient, err := queue.NewClient(dbpool, workers)
	if err != nil {
		return nil, err
	}

	database, err := db.New(cfg, riverClient)
	if err != nil {
		search.Close()
		return nil, err
	}
	river.AddWorker(workers, &qworkers.IndexWorker{DB: database})
	if err := riverClient.Start(ctx); err != nil {
		return nil, err
	}

	srvCtx, cancel := context.WithCancel(ctx)

	go m.Watch(srvCtx, func(ctx context.Context, object, contentType string) {
		args := queue.ProcessArgs{Bucket: m.Bucket, Key: object, ContentType: contentType}
		if err := queue.Enqueue(ctx, riverClient, args); err != nil {
			log.Printf("enqueue process %s: %v", object, err)
		}
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
		Queue:  riverClient,
		DBPool: dbpool,
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
	if s.Queue != nil {
		s.Queue.Stop(context.Background())
	}
	if s.DBPool != nil {
		s.DBPool.Close()
	}
	s.DB.Close()
	search.Close()
}
