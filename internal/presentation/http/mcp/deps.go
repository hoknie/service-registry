package mcp

import (
	apisservices "svc-registry/internal/feature/apis/services"
	catalogservices "svc-registry/internal/feature/catalog/services"
	composeservices "svc-registry/internal/feature/compose/services"
	deployservices "svc-registry/internal/feature/deploy/services"
	gatewayservices "svc-registry/internal/feature/gateway/services"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	linksservices "svc-registry/internal/feature/links/services"
	skillsservices "svc-registry/internal/feature/skills/services"
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
	Skills         *skillsservices.SkillsServices
	Gateway        *gatewayservices.GatewayServices
	PublicURL      string
	Metrics        *metrics.Metrics
	MaxResultBytes int
}
