package service

import (
	"context"
	"time"

	"svc-registry/internal/apperr"
	"svc-registry/internal/postgres"
)

func Ready(ctx context.Context, state *State) error {
	timeout := time.Duration(state.Config.DB.AcquireTimeoutSecs) * time.Second
	if postgres.Ping(ctx, state.DB, timeout) {
		return nil
	}
	return &apperr.Error{Kind: apperr.Unavailable, What: "database"}
}
