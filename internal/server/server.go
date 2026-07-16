package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"era/booru/internal/api"
	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/processing"
	"era/booru/internal/queue"
	"era/booru/internal/storage"
	embedworker "era/booru/internal/workers/embedworker"
	mediaworker "era/booru/internal/workers/mediaworker"

	"github.com/riverqueue/river"
)

// Server bundles initialized services and the Gin router.
type Server struct {
	Router *gin.Engine
	Store  *storage.Store
	Cfg    *config.Config
	Queue  *river.Client[pgx.Tx]
	DBPool *pgxpool.Pool
	ctx    context.Context
	cancel context.CancelFunc
}

// New constructs a Server and starts its background media processor.
func New(ctx context.Context, cfg *config.Config) (*Server, error) {
	store, err := storage.New(cfg.MediaRoot)
	if err != nil {
		return nil, err
	}
	pool, err := db.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}

	workers := river.NewWorkers()
	limiter := processing.NewLimiter(cfg.MediaConcurrency)
	river.AddWorker(workers, &mediaworker.ProcessWorker{Store: store, DB: pool, Limiter: limiter})
	river.AddWorker(workers, &embedworker.ImageEmbedWorker{Store: store, DB: pool, Limiter: limiter})

	riverClient, err := queue.NewClient(ctx, pool, workers, cfg.MediaConcurrency)
	if err != nil {
		pool.Close()
		return nil, err
	}
	if err := riverClient.Start(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	srvCtx, cancel := context.WithCancel(ctx)

	r := gin.New()
	r.Use(api.GinLogger(), gin.Recovery(), api.CORSMiddleware())
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	api.RegisterMediaRoutes(r, pool, store, riverClient, limiter)
	api.RegisterTagRoutes(r, pool)
	api.RegisterAdminRoutes(r, pool, store, riverClient)
	api.RegisterSettingsRoutes(r, pool)
	api.RegisterStaticRoutes(r)

	s := &Server{
		Router: r,
		Store:  store,
		Cfg:    cfg,
		Queue:  riverClient,
		DBPool: pool,
		ctx:    srvCtx,
		cancel: cancel,
	}
	return s, nil
}

// Run starts serving using Gin's Run.
func (s *Server) Run(ctx context.Context, addr string) error {
	httpServer := &http.Server{Addr: addr, Handler: s.Router}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	return httpServer.ListenAndServe()
}

// Close shuts down background resources.
func (s *Server) Close() {
	s.cancel()
	if s.Queue != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.Queue.Stop(ctx)
	}
	if s.DBPool != nil {
		s.DBPool.Close()
	}
}
