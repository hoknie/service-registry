package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/config"
	"svc-registry/internal/webui"
)

func Serve(cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return ServeUntil(ctx, cfg)
}

func ServeUntil(ctx context.Context, cfg config.Config) error {
	addr := cfg.HTTP.Addr.String()
	grace := time.Duration(cfg.HTTP.ShutdownTimeoutSecs) * time.Second
	dist := webui.NewDist(cfg.Web)
	state, err := BuildState(cfg)
	if err != nil {
		return err
	}
	defer state.DB.Close()
	checkCtx, cancelCheck := context.WithTimeout(ctx, 5*time.Second)
	err = CheckSearch(checkCtx, state)
	cancelCheck()
	if err != nil {
		return err
	}
	app := BuildRouter(state, dist)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %w", addr, err)
	}
	slog.Info("listening", "addr", addr, "web_dist", dist.Root())
	if !dist.HasExport() {
		slog.Warn("no web UI export (404.html missing); run `just web-build` or set WEB_DIST_DIR", "web_dist", dist.Root())
	}
	tasks, cancelTasks := context.WithCancel(context.Background())
	defer cancelTasks()
	SpawnEventRetention(tasks, state)
	SpawnBranchRetention(tasks, state)
	SpawnLoginStateRetention(tasks, state)
	forgeDone := SpawnForgeSync(tasks, state)
	linksDone := SpawnLinkChecks(tasks, state)
	clustersDone := SpawnClusterPolls(tasks, state)
	knowledgeDone := SpawnKnowledgeCollection(tasks, state)
	indexDone := SpawnKnowledgeIndex(tasks, state)
	if cfg.Bootstrap.Admin != nil {
		SpawnBootstrapAdmin(tasks, state, *cfg.Bootstrap.Admin)
	}

	served := make(chan error, 1)
	go func() {
		served <- app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})
	}()
	select {
	case err := <-served:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
	case <-ctx.Done():
		slog.Info("shutting down", "grace", grace)
		cancelTasks()
		shutdown, cancel := context.WithTimeout(context.Background(), grace)
		defer cancel()
		select {
		case <-forgeDone:
		case <-shutdown.Done():
			slog.Warn("shutdown grace period elapsed; forge sync still stopping")
		}
		select {
		case <-linksDone:
		case <-shutdown.Done():
			slog.Warn("shutdown grace period elapsed; link checks still stopping")
		}
		select {
		case <-clustersDone:
		case <-shutdown.Done():
			slog.Warn("shutdown grace period elapsed; cluster polls still stopping")
		}
		select {
		case <-knowledgeDone:
		case <-shutdown.Done():
			slog.Warn("shutdown grace period elapsed; documentation collection still stopping")
		}
		select {
		case <-indexDone:
		case <-shutdown.Done():
			slog.Warn("shutdown grace period elapsed; documentation indexing still stopping")
		}
		if err := app.ShutdownWithContext(shutdown); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				slog.Warn("shutdown grace period elapsed; exiting with requests in flight")
			} else {
				slog.Warn("shutdown", "error", err)
			}
		}
	}
	slog.Info("stopped")
	return nil
}
