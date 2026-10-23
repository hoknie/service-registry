package app

import (
	"github.com/google/uuid"

	"svc-registry/internal/feature/deploy/clients/registry"
	deployservices "svc-registry/internal/feature/deploy/services"
	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/outbound"
)

type AgentSettings struct {
	URL      string
	Cluster  uuid.UUID
	Token    string
	Outbound config.OutboundConfig
	K8s      config.K8sClientConfig
}

func OpenAgent(s AgentSettings, opts ...Option) (*deployservices.AgentRunner, error) {
	client, err := k8sFactory(overridesOf(opts), s.Outbound, s.K8s).Client(deploytypes.Access{InCluster: true})
	if err != nil {
		return nil, err
	}
	hc := outbound.New(s.Outbound.Options())
	return deployservices.NewAgentRunner(client, registry.NewRegistryClient(hc, s.URL, s.Cluster, s.Token, Version), Version), nil
}
