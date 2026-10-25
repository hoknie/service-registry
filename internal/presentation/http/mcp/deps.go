package mcp

import (
	apisservices "svc-registry/internal/feature/apis/services"
	catalogservices "svc-registry/internal/feature/catalog/services"
	composeservices "svc-registry/internal/feature/compose/services"
	deployservices "svc-registry/internal/feature/deploy/services"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	linksservices "svc-registry/internal/feature/links/services"
	topologyservices "svc-registry/internal/feature/topology/services"
	"svc-registry/internal/platform/metrics"
)

type Deps struct {
	Catalog        *catalogservices.CatalogServices
	Deploy         *deployservices.DeployServices
	Knowledge      *knowledgeservices.KnowledgeServices
	Links          *linksservices.LinksServices
	Compose        *composeservices.ComposeServices
	Topology       *topologyservices.TopologyServices
	Apis           *apisservices.ApisServices
	Metrics        *metrics.Metrics
	MaxResultBytes int
}
