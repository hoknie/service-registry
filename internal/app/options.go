package app

import (
	"svc-registry/internal/feature/deploy/clients/k8s"
	deployports "svc-registry/internal/feature/deploy/ports"
	linksports "svc-registry/internal/feature/links/ports"
	"svc-registry/internal/platform/config"
)

type Option func(*overrides)

type overrides struct {
	linkChecker linksports.CheckerPort
	k8s         deployports.ClientFactoryPort
}

func WithLinkChecker(c linksports.CheckerPort) Option {
	return func(o *overrides) { o.linkChecker = c }
}

func WithK8s(f deployports.ClientFactoryPort) Option { return func(o *overrides) { o.k8s = f } }

func k8sFactory(o overrides, cfg config.Config) deployports.ClientFactoryPort {
	if o.k8s != nil {
		return o.k8s
	}
	return k8s.NewFactoryClient(cfg.Outbound, cfg.K8s)
}

func overridesOf(opts []Option) overrides {
	var o overrides
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
