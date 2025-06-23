package db

import (
	"context"
	"fmt"

	"era/booru/ent"
	"era/booru/ent/hook"
	"era/booru/ent/migrate"
	_ "era/booru/ent/runtime"
	"era/booru/internal/config"

	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/lib/pq"
)

// New creates a new ent.Client connected to Postgres and runs migrations.
func New(cfg *config.Config) (*ent.Client, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)

	client, err := ent.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	opts := []schema.MigrateOption{}
	if cfg.DevMode {
		opts = append(opts,
			migrate.WithDropColumn(true),
			migrate.WithDropIndex(true),
		)
	}
	if err := client.Schema.Create(context.Background(), opts...); err != nil {
		return nil, err
	}
	client.Media.Use(hook.SyncBleve())
	return client, nil
}
