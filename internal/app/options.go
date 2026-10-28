package app

import (
	membershipports "svc-registry/internal/feature/membership/ports"
	"time"

	"svc-registry/internal/feature/access/clients/clientmeta"
	accessports "svc-registry/internal/feature/access/ports"
	"svc-registry/internal/feature/deploy/clients/k8s"
	deployports "svc-registry/internal/feature/deploy/ports"
	gatewayports "svc-registry/internal/feature/gateway/ports"
	linksports "svc-registry/internal/feature/links/ports"
	"svc-registry/internal/feature/notify/clients/sender"
	notifyports "svc-registry/internal/feature/notify/ports"
	skillsports "svc-registry/internal/feature/skills/ports"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/metrics"
)

type Option func(*overrides)

type overrides struct {
	mailer         membershipports.MailerPort
	linkChecker    linksports.CheckerPort
	k8s            deployports.ClientFactoryPort
	notifySender   notifyports.SenderPort
	clientMetadata accessports.ClientMetadataPort
	metricSources  []metrics.Source
	skillsStore    skillsports.BlobStorePort
	gatewayClient  gatewayports.ClientPort
}

func WithLinkChecker(c linksports.CheckerPort) Option {
	return func(o *overrides) { o.linkChecker = c }
}

func WithNotifySender(s notifyports.SenderPort) Option {
	return func(o *overrides) { o.notifySender = s }
}

func notifySender(o overrides, cfg config.Config) notifyports.SenderPort {
	if o.notifySender != nil {
		return o.notifySender
	}
	timeout := time.Duration(cfg.Notify.TimeoutSecs) * time.Second
	if cfg.Notify.UseProxy {
		return sender.NewProxied(cfg.Outbound.Options(), timeout, cfg.Notify.Policy())
	}
	return sender.New(cfg.Outbound.Options(), timeout, cfg.Notify.Policy())
}

func WithClientMetadata(f accessports.ClientMetadataPort) Option {
	return func(o *overrides) { o.clientMetadata = f }
}

func clientMetadata(o overrides, cfg config.Config) accessports.ClientMetadataPort {
	if o.clientMetadata != nil {
		return o.clientMetadata
	}
	if cfg.MCPOAuth.ClientMetadataUseProxy {
		return clientmeta.NewProxied(cfg.Outbound.Options())
	}
	return clientmeta.New(cfg.Outbound.Options())
}

func WithMailer(m membershipports.MailerPort) Option { return func(o *overrides) { o.mailer = m } }

func WithK8s(f deployports.ClientFactoryPort) Option { return func(o *overrides) { o.k8s = f } }

func k8sFactory(o overrides, out config.OutboundConfig, k config.K8sClientConfig) deployports.ClientFactoryPort {
	if o.k8s != nil {
		return o.k8s
	}
	return k8s.NewFactoryClient(out, k)
}

func overridesOf(opts []Option) overrides {
	var o overrides
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

func WithMetricsSource(s metrics.Source) Option {
	return func(o *overrides) { o.metricSources = append(o.metricSources, s) }
}
