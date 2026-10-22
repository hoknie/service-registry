package handlers

import (
	"sync"

	"github.com/gofiber/fiber/v3"

	accessservices "svc-registry/internal/feature/access/services"
	catalogservices "svc-registry/internal/feature/catalog/services"
	composeservices "svc-registry/internal/feature/compose/services"
	deployservices "svc-registry/internal/feature/deploy/services"
	forgeservices "svc-registry/internal/feature/forge/services"
	ingestservices "svc-registry/internal/feature/ingest/services"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	linksservices "svc-registry/internal/feature/links/services"
	"svc-registry/internal/platform/config"
)

type Handlers struct {
	Config    *config.Config
	Readiness Readiness
	Access    *accessservices.AccessServices
	Catalog   *catalogservices.CatalogServices
	Links     *linksservices.LinksServices
	Ingest    *ingestservices.IngestServices
	Forge     *forgeservices.ForgeServices
	Deploy    *deployservices.DeployServices
	Knowledge *knowledgeservices.KnowledgeServices
	Compose   *composeservices.ComposeServices
	Version   string

	mcpOnce    sync.Once
	mcpHandler fiber.Handler
}
