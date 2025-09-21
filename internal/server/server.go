package server

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"era/booru/ent"
	"era/booru/internal/api"
	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/minio"
	"era/booru/internal/queue"
	"era/booru/internal/search"
	indexworker "era/booru/internal/workers/indexworker"
	mediaworker "era/booru/internal/workers/mediaworker"

	"github.com/riverqueue/river"
)

// Server bundles initialized services and the Gin router.
type Server struct {
	Router *gin.Engine
	DB     *ent.Client
	Minio  *minio.Client
	Cfg    *config.Config
	Queue  *river.Client[pgx.Tx] // Changed from *sql.Tx
	DBPool *pgxpool.Pool         // Changed from *sql.DB
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
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		search.Close()
		return nil, err
	}

	riverClient, err := queue.NewClient(ctx, pool, workers, queue.ClientTypeServer)
	if err != nil {
		search.Close()
		return nil, err
	}

	database, err := db.New(cfg, riverClient, true)
	if err != nil {
		search.Close()
		return nil, err
	}

	river.AddWorker(workers, &indexworker.IndexWorker{DB: database})
	if err := riverClient.Start(ctx); err != nil {
		return nil, err
	}
	river.AddWorker(workers, &mediaworker.ProcessWorker{
		Minio: m,
		DB:    database,
		Cfg:   cfg,
	})

	srvCtx, cancel := context.WithCancel(ctx)

	go m.Watch(srvCtx, func(ctx context.Context, object, contentType string) {
		log.Printf("object %s uploaded with content type %s", object, contentType)
		args := queue.ProcessArgs{Bucket: m.Bucket, Key: object, ContentType: contentType}
		if err := queue.Enqueue(ctx, riverClient, args); err != nil {
			log.Printf("enqueue process %s: %v", object, err)
		}
	})

	r := gin.New()
	r.Use(api.GinLogger(), gin.Recovery(), api.CORSMiddleware())
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	api.RegisterMediaRoutes(r, database, m, cfg)
	api.RegisterTagRoutes(r, database)
	api.RegisterAdminRoutes(r, database, m, cfg, riverClient)
	api.RegisterStaticRoutes(r)

	s := &Server{
		Router: r,
		DB:     database,
		Minio:  m,
		Cfg:    cfg,
		Queue:  riverClient,
		DBPool: pool,
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
