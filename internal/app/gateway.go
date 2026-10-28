package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/feature/gateway/clients/mcpclient"
	gatewayports "svc-registry/internal/feature/gateway/ports"
	gatewayservices "svc-registry/internal/feature/gateway/services"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/metrics"
	"svc-registry/pkg/netguard"
)

func WithGatewayClient(c gatewayports.ClientPort) Option {
	return func(o *overrides) { o.gatewayClient = c }
}

func gatewayClient(o overrides, cfg config.Config) gatewayports.ClientPort {
	if o.gatewayClient != nil {
		return o.gatewayClient
	}
	direct := cfg.Outbound.Options()
	direct.HTTPSProxy, direct.HTTPProxy, direct.NoProxy = "", "", ""
	return mcpclient.New(mcpclient.Options{Outbound: direct, Environment: cfg.Outbound.Options(),
		Policy:    netguard.Policy{AllowHosts: cfg.MCPGateway.AllowHosts, AllowNets: cfg.MCPGateway.AllowNets},
		UserAgent: "svc-registry/" + Version, Version: Version, MaxTools: int(cfg.MCPGateway.MaxTools)})
}

func (a *App) buildGateway(o overrides) {
	if !a.Config.MCPGateway.Enabled {
		return
	}
	a.Gateway = gatewayservices.New(gatewayservices.Deps{DB: a.DB, Config: a.Config, Catalog: a.Catalog,
		Client: instrument(a.Metrics, "mcp_gateway", gatewayClient(o, *a.Config)), Observer: gatewayObserver{m: a.Metrics}})
}

type gatewayObserver struct{ m *metrics.Metrics }

func (g gatewayObserver) CheckDone(outcome string) { g.m.MCPGatewayCheck(outcome) }

func (g gatewayObserver) CallDone(server uuid.UUID, outcome string, seconds float64) {
	g.m.MCPGatewayCall(server.String(), outcome, time.Duration(seconds*float64(time.Second)))
}

func (a *App) gatewaySamples(ctx context.Context) ([]metrics.Sample, error) {
	if a.Gateway == nil {
		return nil, nil
	}
	return a.Gateway.Ports.MetricSamples(ctx)
}
