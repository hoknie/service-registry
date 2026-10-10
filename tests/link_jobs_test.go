package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/presentation/jobs"
)

func init() { jobs.LinkCheckTick = 50 * time.Millisecond }

func TestOneAddressOfTwoProjectsIsCheckedOnce(t *testing.T) {
	t.Parallel()
	tr := newLinkTree(t)
	app, root := tr.app, tr.root
	app.allowLoopbackChecks()
	tg := newTarget(t, 200)
	app.putTemplate(root, tr.org, "runbook", tg.URL+"/runbook")
	second := appOver(t, app.db)
	second.allowLoopbackChecks()

	ctx, cancel := context.WithCancel(context.Background())
	done1 := jobs.SpawnLinkChecks(ctx, app.services.Jobs())
	done2 := jobs.SpawnLinkChecks(ctx, second.services.Jobs())
	waitFor(t, "a check", func() bool { return app.count("link_checks") > 0 })
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done1
	<-done2
	eq(t, tg.hits.Load(), int32(1))
	eq(t, app.count("link_targets"), int64(1))
	eq(t, app.count("link_checks"), int64(1))
	eq(t, scalar[bool](t, app.db, "SELECT lease_until IS NULL AND next_run_at > now() + interval '50 minutes' FROM link_targets"), true)
	for _, project := range []string{tr.project, tr.other} {
		eq(t, at(app.links(root, project, "")[0], "check", "status"), any("ok"), project)
	}
}

func TestUnseenAddressesAreDeletedWithTheirHistory(t *testing.T) {
	t.Parallel()
	tr := newLinkTree(t)
	app := tr.app
	old, fresh := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, app.db, "INSERT INTO link_targets (id, url, last_seen_at, next_run_at) VALUES ($1, 'https://old.example/', now() - interval '31 days', now() + interval '1 day')", old)
	execSQL(t, app.db, "INSERT INTO link_targets (id, url, last_seen_at, next_run_at) VALUES ($1, 'https://fresh.example/', now() - interval '29 days', now() + interval '1 day')", fresh)
	execSQL(t, app.db, "INSERT INTO link_checks (id, target_id, status, duration_ms) VALUES ($1, $2, 'ok', 1)", uuid.Must(uuid.NewV7()), old)
	ctx, cancel := context.WithCancel(context.Background())
	done := jobs.SpawnLinkChecks(ctx, app.services.Jobs())
	waitFor(t, "the prune", func() bool { return app.count("link_targets") == 1 })
	cancel()
	<-done
	eq(t, scalar[string](t, app.db, "SELECT url FROM link_targets"), "https://fresh.example/")
	eq(t, app.count("link_checks"), int64(0))
}

func TestLinkChecksStopWithBackgroundJobs(t *testing.T) {
	t.Parallel()
	tdb := newLinkTree(t).app.db
	app := appOver(t, tdb, "BACKGROUND_JOBS_ENABLED", "false")
	app.allowLoopbackChecks()
	root := app.session("root@example.com", password)
	org := scalar[string](t, tdb, "SELECT id::text FROM nodes WHERE slug = 'acme'")
	project := scalar[string](t, tdb, "SELECT id::text FROM nodes WHERE slug = 'api'")
	tg := newTarget(t, 200)
	app.putTemplate(root, org, "runbook", tg.URL+"/")
	eq(t, len(app.links(root, project, "")), 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := jobs.SpawnLinkChecks(ctx, app.services.Jobs())
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
	eq(t, tg.hits.Load(), int32(0))
	eq(t, app.count("link_checks"), int64(0))
	eq(t, at(list(t, app.call("POST", linksPath(project)+"/runbook/check", root).json(t), "items")[0], "check", "status"), any("ok"))
}
