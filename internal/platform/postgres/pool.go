package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"svc-registry/internal/platform/config"
)

func ConnectLazy(cfg config.DbConfig) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}
	pc.MaxConns = int32(cfg.MaxConnections)
	pc.MinConns = 0
	pc.ConnConfig.ConnectTimeout = time.Duration(cfg.AcquireTimeoutSecs) * time.Second
	pool, err := pgxpool.NewWithConfig(context.Background(), pc)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}
	return pool, nil
}
