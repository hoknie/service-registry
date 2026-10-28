package jobs

import (
	accessservices "svc-registry/internal/feature/access/services"
	catalogservices "svc-registry/internal/feature/catalog/services"
	deployservices "svc-registry/internal/feature/deploy/services"
	gatewayservices "svc-registry/internal/feature/gateway/services"
	ingestservices "svc-registry/internal/feature/ingest/services"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	linksservices "svc-registry/internal/feature/links/services"
	membershipservices "svc-registry/internal/feature/membership/services"
	notifyservices "svc-registry/internal/feature/notify/services"
	reposyncservices "svc-registry/internal/feature/reposync/services"
	skillsservices "svc-registry/internal/feature/skills/services"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/metrics"
)

type Deps struct {
	Config     *config.Config
	Access     *accessservices.AccessServices
	Catalog    *catalogservices.CatalogServices
	Links      *linksservices.LinksServices
	Ingest     *ingestservices.IngestServices
	RepoSync   *reposyncservices.RepoSyncServices
	Deploy     *deployservices.DeployServices
	Knowledge  *knowledgeservices.KnowledgeServices
	Notify     *notifyservices.NotifyServices
	Skills     *skillsservices.SkillsServices
	Gateway    *gatewayservices.GatewayServices
	Membership *membershipservices.MembershipServices
	Metrics    *metrics.Metrics
}
