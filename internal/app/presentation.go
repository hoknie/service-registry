package app

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/platform/postgres"

	apphttp "svc-registry/internal/presentation/http"
	"svc-registry/internal/presentation/http/webui"
	"svc-registry/internal/presentation/jobs"
)

func (a *App) Router(dist webui.Dist) *fiber.App {
	return apphttp.NewRouter(apphttp.Deps{Config: a.Config, Readiness: databaseReadiness{a}, Version: Version, Access: a.Access, Catalog: a.Catalog,
		Links: a.Links, Ingest: a.Ingest, Forge: a.Forge, Deploy: a.Deploy, Knowledge: a.Knowledge, Compose: a.Compose}, dist)
}

func (a *App) Jobs() jobs.Deps {
	return jobs.Deps{Config: a.Config, Access: a.Access, Catalog: a.Catalog, Links: a.Links, Ingest: a.Ingest,
		Forge: a.Forge, Deploy: a.Deploy, Knowledge: a.Knowledge}
}

type databaseReadiness struct{ app *App }

func (r databaseReadiness) Ready(ctx context.Context) bool {
	timeout := time.Duration(r.app.Config.DB.AcquireTimeoutSecs) * time.Second
	return postgres.Ping(ctx, r.app.DB.Pool(), timeout)
}
