package server

import (
        "context"
        "log"
        "net/http"

        "github.com/gin-gonic/gin"
        "github.com/jackc/pgx/v5"
        "github.com/jackc/pgx/v5/pgxpool"
        "github.com/riverqueue/river"
        "github.com/riverqueue/river/riverdriver/riverpgxv5"

        "era/booru/ent"
        "era/booru/internal/api"
        "era/booru/internal/config"
        "era/booru/internal/db"
        "era/booru/internal/minio"
        "era/booru/internal/search"
        "era/booru/internal/tasks"
)

// Server bundles initialized services and the Gin router.
type Server struct {
        Router *gin.Engine
        DB     *ent.Client
        Minio  *minio.Client
       River  *river.Client[pgx.Tx]
       Pool   *pgxpool.Pool
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

       pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
       if err != nil {
               search.Close()
               return nil, err
       }

       workers := river.NewWorkers()
       river.AddWorker(workers, &tasks.AnalyzeWorker{Cfg: cfg, Minio: m, DB: database})

       riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
               Queues: map[string]river.QueueConfig{
                       river.QueueDefault: {MaxWorkers: 5},
               },
               Workers: workers,
       })
       if err != nil {
               search.Close()
               return nil, err
       }
       if err := riverClient.Start(ctx); err != nil {
               search.Close()
               return nil, err
       }

       srvCtx, cancel := context.WithCancel(ctx)

       go m.Watch(srvCtx, func(ctx context.Context, object, contentType string) {
               if _, err := riverClient.Insert(ctx, tasks.AnalyzeArgs{Object: object, ContentType: contentType}, nil); err != nil {
                       log.Printf("enqueue %s: %v", object, err)
               }
       })

	r := gin.New()
	r.Use(api.GinLogger(), gin.Recovery(), api.CORSMiddleware())
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusNoContent) })
       api.RegisterMediaRoutes(r, database, m, cfg)
       api.RegisterAdminRoutes(r, database, m, cfg, riverClient)
       api.RegisterStaticRoutes(r)

       s := &Server{
               Router: r,
               DB:     database,
               Minio:  m,
               River:  riverClient,
               Pool:   pool,
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
       if s.River != nil {
               _ = s.River.Stop(context.Background())
       }
        s.DB.Close()
       if s.Pool != nil {
               s.Pool.Close()
       }
        search.Close()
}
