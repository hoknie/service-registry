package handlers

import (
	"sync"

	"github.com/gofiber/fiber/v3"

	accessservices "svc-registry/internal/feature/access/services"
	apisservices "svc-registry/internal/feature/apis/services"
	catalogservices "svc-registry/internal/feature/catalog/services"
	composeservices "svc-registry/internal/feature/compose/services"
	deployservices "svc-registry/internal/feature/deploy/services"
	gatewayservices "svc-registry/internal/feature/gateway/services"
	ingestservices "svc-registry/internal/feature/ingest/services"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	linksservices "svc-registry/internal/feature/links/services"
	membershipservices "svc-registry/internal/feature/membership/services"
	notifyservices "svc-registry/internal/feature/notify/services"
	ownershipservices "svc-registry/internal/feature/ownership/services"
	publishingservices "svc-registry/internal/feature/publishing/services"
	reposyncservices "svc-registry/internal/feature/reposync/services"
	skillsservices "svc-registry/internal/feature/skills/services"
	topologyservices "svc-registry/internal/feature/topology/services"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/metrics"
)

type Handlers struct {
	Config     *config.Config
	Readiness  Readiness
	Access     *accessservices.AccessServices
	Catalog    *catalogservices.CatalogServices
	Links      *linksservices.LinksServices
	Ingest     *ingestservices.IngestServices
	RepoSync   *reposyncservices.RepoSyncServices
	Deploy     *deployservices.DeployServices
	Knowledge  *knowledgeservices.KnowledgeServices
	Ownership  *ownershipservices.OwnershipServices
	Notify     *notifyservices.NotifyServices
	Topology   *topologyservices.TopologyServices
	Apis       *apisservices.ApisServices
	Compose    *composeservices.ComposeServices
	Publishing *publishingservices.PublishingServices
	Skills     *skillsservices.SkillsServices
	Gateway    *gatewayservices.GatewayServices
	Membership *membershipservices.MembershipServices
	Metrics    *metrics.Metrics
	Version    string

	mcpOnce    sync.Once
	mcpHandler fiber.Handler
}
