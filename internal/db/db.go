package db

import (
	"context"

	"era/booru/ent"
	"era/booru/ent/hook"
	"era/booru/ent/migrate"
	_ "era/booru/ent/runtime"
	"era/booru/internal/config"

	"entgo.io/ent/dialect/sql/schema"
	"github.com/jackc/pgx/v5"
	_ "github.com/lib/pq"
	"github.com/riverqueue/river"
)

// New creates a new ent.Client connected to Postgres and runs migrations.
func New(cfg *config.Config, q *river.Client[pgx.Tx]) (*ent.Client, error) {
	dsn := cfg.PostgresDSN

	client, err := ent.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	// Auto migrate (Later: make it work like that only in devmode)
	opts := []schema.MigrateOption{}
	opts = append(opts,
		migrate.WithDropColumn(true),
		migrate.WithDropIndex(true),
	)

	if err := client.Schema.Create(context.Background(), opts...); err != nil {
		return nil, err
	}
	client.Media.Use(hook.SyncBleve(q))
	return client, nil
}
