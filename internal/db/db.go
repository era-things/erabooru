package db

import (
	"context"
	"fmt"

	"era/booru/ent"
	_ "era/booru/ent/runtime"
	"era/booru/internal/config"

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
	if err := client.Schema.Create(context.Background()); err != nil {
		return nil, err
	}
	return client, nil
}
