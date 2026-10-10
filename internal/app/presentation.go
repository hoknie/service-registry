package app

import (
	"github.com/gofiber/fiber/v3"

	apphttp "svc-registry/internal/presentation/http"
	"svc-registry/internal/presentation/http/handlers"
	"svc-registry/internal/presentation/http/webui"
	"svc-registry/internal/presentation/jobs"
)

func (a *App) Handlers() *handlers.Handlers {
	return &handlers.Handlers{Config: a.Config, DB: a.DB, Access: a.Access, Catalog: a.Catalog, Links: a.Links,
		Ingest: a.Ingest, Forge: a.Forge, Deploy: a.Deploy, Knowledge: a.Knowledge}
}

func (a *App) Router(dist webui.Dist) *fiber.App { return apphttp.NewRouter(a.Handlers(), dist) }

func (a *App) Jobs() jobs.Deps {
	return jobs.Deps{Config: a.Config, Access: a.Access, Catalog: a.Catalog, Links: a.Links, Ingest: a.Ingest,
		Forge: a.Forge, Deploy: a.Deploy, Knowledge: a.Knowledge}
}
