package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"svc-registry/pkg/leasejobs"
)

var (
	MCPChecksTick         = 5 * time.Second
	MCPCallRetentionEvery = time.Hour
)

type mcpCheck struct{ deps Deps }

func (s mcpCheck) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	return s.deps.Gateway.Checks.ClaimChecks(ctx, limit, leaseSecs)
}

func (s mcpCheck) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return s.deps.Gateway.Checks.ExtendCheck(ctx, id, leaseSecs)
}

func SpawnMCPChecks(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	if deps.Gateway == nil || !deps.Config.Jobs.Enabled {
		close(done)
		return done
	}
	s := &leasejobs.Scheduler[uuid.UUID]{
		Name:   "mcp-checks",
		Source: mcpCheck{deps: deps},
		Run: func(ctx context.Context, id uuid.UUID) {
			_ = observe(deps, "mcp_checks", func() error { return deps.Gateway.Checks.RunCheck(ctx, id) })
		},
		OnPanic:     panicked(deps, "mcp_checks"),
		Concurrency: 4,
		Tick:        MCPChecksTick,
	}
	go func() {
		defer close(done)
		s.Serve(ctx)
	}()
	return done
}

func SpawnMCPCallRetention(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	if deps.Gateway == nil || !deps.Config.Jobs.Enabled {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		tick := time.NewTicker(MCPCallRetentionEvery)
		defer tick.Stop()
		for {
			n, err := observeCount(deps, "mcp_call_retention", func() (int, error) {
				n, err := deps.Gateway.Ports.PruneCalls(ctx)
				return int(n), err
			})
			switch {
			case ctx.Err() != nil:
				return
			case err != nil:
				slog.Warn("MCP call log cleanup failed; retrying at the next tick", "error", err)
			case n > 0:
				slog.Info("MCP call log cleanup: old calls deleted", "deleted", n)
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	return done
}
