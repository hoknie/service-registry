package postgres

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Ping(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var one int32
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		if ctx.Err() != nil {
			slog.Warn("database ping timed out", "timeout", timeout)
		} else {
			slog.Warn("database ping failed", "error", err)
		}
		return false
	}
	return one == 1
}
