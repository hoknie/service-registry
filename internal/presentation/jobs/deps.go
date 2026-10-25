package jobs

import (
	accessservices "svc-registry/internal/feature/access/services"
	catalogservices "svc-registry/internal/feature/catalog/services"
	deployservices "svc-registry/internal/feature/deploy/services"
	forgeservices "svc-registry/internal/feature/forge/services"
	ingestservices "svc-registry/internal/feature/ingest/services"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	linksservices "svc-registry/internal/feature/links/services"
	notifyservices "svc-registry/internal/feature/notify/services"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/metrics"
)

type Deps struct {
	Config    *config.Config
	Access    *accessservices.AccessServices
	Catalog   *catalogservices.CatalogServices
	Links     *linksservices.LinksServices
	Ingest    *ingestservices.IngestServices
	Forge     *forgeservices.ForgeServices
	Deploy    *deployservices.DeployServices
	Knowledge *knowledgeservices.KnowledgeServices
	Notify    *notifyservices.NotifyServices
	Metrics   *metrics.Metrics
}
