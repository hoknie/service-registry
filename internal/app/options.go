package app

import (
	"svc-registry/internal/feature/deploy"
	"svc-registry/internal/feature/deploy/k8s"
	"svc-registry/internal/feature/links"
	"svc-registry/internal/platform/config"
)

type Option func(*overrides)

type overrides struct {
	linkChecker links.Checker
	k8s         deploy.ClientFactory
}

func WithLinkChecker(c links.Checker) Option { return func(o *overrides) { o.linkChecker = c } }

func WithK8s(f deploy.ClientFactory) Option { return func(o *overrides) { o.k8s = f } }

func k8sFactory(o overrides, cfg config.Config) deploy.ClientFactory {
	if o.k8s != nil {
		return o.k8s
	}
	return k8s.NewFactory(cfg.Outbound, cfg.K8s)
}

func overridesOf(opts []Option) overrides {
	var o overrides
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
