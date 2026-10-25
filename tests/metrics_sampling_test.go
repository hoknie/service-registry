package tests

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	svcapp "svc-registry/internal/app"
	"svc-registry/internal/platform/metrics"
)

func TestSamplesCarryCatalogFiguresAndQueues(t *testing.T) {
	t.Parallel()
	app := startApp(t, "METRICS_ENABLED", "true")
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	backend := app.nodeID(admin, "folder", acme, "backend")
	api := app.nodeID(admin, "project", backend, "api")
	app.nodeID(admin, "project", acme, "web")
	other := app.nodeID(admin, "organization", "", "other")
	app.nodeID(admin, "project", other, "tool")
	execSQL(t, app.db, "DELETE FROM node_owners WHERE node_id = $1", other)
	for i, lease := range []string{"NULL", "NULL", "now() + interval '1 minute'"} {
		execSQL(t, app.db, fmt.Sprintf("INSERT INTO link_targets (id, url, next_run_at, lease_until) VALUES ($1, $2, now() - interval '1 minute', %s)", lease),
			uuid.Must(uuid.NewV7()), fmt.Sprintf("https://x%d.example/", i))
	}
	before := app.metrics("")
	lacks(t, before, "svcr_catalog_nodes")

	app.services.Sampler.SampleNow(context.Background())
	body := app.metrics("")
	eq(t, series(t, body, `svcr_catalog_nodes{kind="organization"}`), 2.0)
	eq(t, series(t, body, `svcr_catalog_nodes{kind="folder"}`), 1.0)
	eq(t, series(t, body, `svcr_catalog_nodes{kind="project"}`), 3.0)
	eq(t, series(t, body, `svcr_projects_covered{aspect="owners"}`), 2.0)
	eq(t, series(t, body, `svcr_users{state="active"}`), 1.0)
	eq(t, series(t, body, `svcr_job_queue{job="link_checks",state="due"}`), 2.0)
	eq(t, series(t, body, `svcr_job_queue{job="link_checks",state="leased"}`), 1.0)
	eq(t, series(t, body, `svcr_link_targets{state="unchecked"}`), 3.0)
	eq(t, series(t, body, fmt.Sprintf(`svcr_project_info{organization_id="%s",path="acme/backend/api",project_id="%s"}`, acme, api)), 1.0)
	contains(t, body, "go_goroutines")
	contains(t, body, "svcr_db_pool_max_connections")
	contains(t, body, `svcr_metrics_sample_timestamp_seconds{source="catalog"}`)
	contains(t, body, `svcr_metrics_sample_timestamp_seconds{source="jobs"}`)
	lacks(t, body, "root@example.com")
}

func TestAFailedSampleKeepsTheLastValues(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	var broken atomic.Bool
	probe := metrics.SampleSource("probe", func(context.Context) ([]metrics.Sample, error) {
		calls.Add(1)
		if broken.Load() {
			return nil, errors.New("database is down")
		}
		return []metrics.Sample{{Name: "svcr_catalog_nodes", Labels: map[string]string{"kind": "probe"}, Value: 7}}, nil
	})
	app := startAppWith(t, []svcapp.Option{svcapp.WithMetricsSource(probe)}, "METRICS_ENABLED", "true")
	app.services.Sampler.SampleNow(context.Background())
	first := app.metrics("")
	eq(t, series(t, first, `svcr_catalog_nodes{kind="probe"}`), 7.0)
	for range 100 {
		app.metrics("")
	}
	eq(t, calls.Load(), int32(1))

	broken.Store(true)
	app.services.Sampler.SampleNow(context.Background())
	second := app.metrics("")
	eq(t, series(t, second, `svcr_catalog_nodes{kind="probe"}`), 7.0)
	eq(t, series(t, second, `svcr_metrics_sample_errors_total{source="probe"}`), 1.0)
	stamp := `svcr_metrics_sample_timestamp_seconds{source="probe"}`
	eq(t, series(t, second, stamp), series(t, first, stamp))
}
