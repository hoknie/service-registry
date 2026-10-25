package app

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"

	apphttp "svc-registry/internal/presentation/http"
	"svc-registry/internal/presentation/http/webui"
	"svc-registry/internal/presentation/jobs"
	"svc-registry/pkg/pgtx"
)

func (a *App) Router(dist webui.Dist) *fiber.App {
	return apphttp.NewRouter(apphttp.Deps{Config: a.Config, Readiness: databaseReadiness{a}, Version: Version, Access: a.Access, Catalog: a.Catalog,
		Links: a.Links, Ingest: a.Ingest, Forge: a.Forge, Deploy: a.Deploy, Knowledge: a.Knowledge,
		Ownership: a.Ownership, Notify: a.Notify, Topology: a.Topology, Apis: a.Apis, Compose: a.Compose, Metrics: a.Metrics}, dist)
}

func (a *App) Jobs() jobs.Deps {
	return jobs.Deps{Config: a.Config, Access: a.Access, Catalog: a.Catalog, Links: a.Links, Ingest: a.Ingest,
		Forge: a.Forge, Deploy: a.Deploy, Knowledge: a.Knowledge, Notify: a.Notify, Metrics: a.Metrics}
}

type databaseReadiness struct{ app *App }

func (r databaseReadiness) Ready(ctx context.Context) bool {
	timeout := time.Duration(r.app.Config.DB.AcquireTimeoutSecs) * time.Second
	return pgtx.Ping(ctx, r.app.DB.Pool(), timeout)
}
