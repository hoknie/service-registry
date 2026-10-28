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

	"svc-registry/internal/platform/config"
	"svc-registry/internal/presentation/http/webui"
	"svc-registry/internal/presentation/jobs"
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
	registry, err := New(cfg)
	if err != nil {
		return err
	}
	defer registry.Close()
	checkCtx, cancelCheck := context.WithTimeout(ctx, 5*time.Second)
	err = registry.CheckSearch(checkCtx)
	cancelCheck()
	if err != nil {
		return err
	}
	registry.CheckSkillsStorage(ctx)
	app := registry.Router(dist)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %w", addr, err)
	}
	slog.Info("listening", "addr", addr, "web_dist", dist.Root())
	if !dist.HasExport() {
		slog.Warn("no web UI export (404.html missing); run `just web-build` or set WEB_DIST_DIR", "web_dist", dist.Root())
	}
	if cfg.Metrics.Enabled && cfg.Metrics.Token == "" {
		slog.Warn("metrics are exposed at /metrics without METRICS_TOKEN")
	}
	if cfg.Notify.UseProxy {
		slog.Warn("NOTIFY_USE_PROXY=true: notifications go through the outbound proxy; the receiver's IP behind the proxy is not checked")
	}
	if cfg.MCPOAuth.Enabled && cfg.MCPOAuth.ClientMetadata {
		if !cfg.MCPOAuth.ClientDocuments() {
			slog.Info("client metadata documents are refused: MCP_OAUTH_CLIENT_METADATA_HOSTS lists no hosts")
		}
		if cfg.MCPOAuth.ClientMetadataAnyHost {
			slog.Warn("MCP_OAUTH_CLIENT_METADATA_HOSTS=*: client metadata documents are accepted from any host that passes the address policy")
		}
		if cfg.MCPOAuth.ClientDocuments() && cfg.MCPOAuth.ClientMetadataUseProxy {
			slog.Warn("MCP_OAUTH_CLIENT_METADATA_USE_PROXY=true: client metadata documents are fetched through the outbound proxy; the IP behind the proxy is not checked")
		}
	}
	tasks, cancelTasks := context.WithCancel(context.Background())
	deps := registry.Jobs()
	defer cancelTasks()
	jobs.SpawnEventRetention(tasks, deps)
	jobs.SpawnBranchRetention(tasks, deps)
	jobs.SpawnKnowledgeBlobRetention(tasks, deps)
	jobs.SpawnLinkIconRetention(tasks, deps)
	jobs.SpawnLoginStateRetention(tasks, deps)
	jobs.SpawnOAuthRetention(tasks, deps)
	jobs.SpawnInvitationRetention(tasks, deps)
	forgeDone := jobs.SpawnForgeSync(tasks, deps)
	linksDone := jobs.SpawnLinkChecks(tasks, deps)
	clustersDone := jobs.SpawnClusterPolls(tasks, deps)
	knowledgeDone := jobs.SpawnKnowledgeCollection(tasks, deps)
	indexDone := jobs.SpawnKnowledgeIndex(tasks, deps)
	notifyDone := jobs.SpawnNotifyDeliveries(tasks, deps)
	skillsDone := jobs.SpawnSkillSources(tasks, deps)
	jobs.SpawnSkillBlobRetention(tasks, deps)
	gatewayDone := jobs.SpawnMCPChecks(tasks, deps)
	jobs.SpawnMCPCallRetention(tasks, deps)
	if registry.Sampler != nil {
		go registry.Sampler.Serve(tasks)
	}
	if cfg.Bootstrap.Admin != nil {
		jobs.SpawnBootstrapAdmin(tasks, deps, *cfg.Bootstrap.Admin)
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
		select {
		case <-notifyDone:
		case <-shutdown.Done():
			slog.Warn("shutdown grace period elapsed; notification deliveries still stopping")
		}
		select {
		case <-gatewayDone:
		case <-shutdown.Done():
			slog.Warn("shutdown grace period elapsed; MCP server checks still stopping")
		}
		select {
		case <-skillsDone:
		case <-shutdown.Done():
			slog.Warn("shutdown grace period elapsed; skill source syncs still stopping")
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
