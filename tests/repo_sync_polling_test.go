package tests

import (
	"testing"
	"time"

	"svc-registry/pkg/testing/hostingfake"
)

func secondsUntilNextRun(t *testing.T, app *testApp, conn string) float64 {
	t.Helper()
	return scalar[float64](t, app.db, "SELECT extract(epoch FROM next_run_at - now())::float8 FROM repo_sync_connections WHERE id = $1", conn)
}

func runCount(t *testing.T, app *testApp, cookie, node, conn string) int {
	t.Helper()
	return len(list(t, app.get(connectionPath(node, conn)+"/runs", cookie).json(t), "items"))
}

func TestRepoSyncIntervalComesFromPollingSettings(t *testing.T) {
	t.Parallel()
	app := startRepoSyncApp(t, "REPO_SYNC_INTERVAL_SECS", "900")
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := hostingfake.Start(t, "github", hostingToken, "acme-inc")
	f.Put(hostingfake.Repo{ID: 1, Path: "acme-inc/api"})

	created := app.connect(root, acme, f, nil)
	conn := idOf(created)
	eq(t, created["interval_secs"], any(nil))
	eq(t, created["effective_interval_secs"], any(float64(900)))
	eq(t, created["paused"], any(false))
	app.runDue()
	eq(t, runCount(t, app, root, acme, conn), 1)
	if s := secondsUntilNextRun(t, app, conn); s < 800 || s > 900 {
		t.Fatalf("next run in %v s, want about 900", s)
	}

	eq(t, app.send("PUT", pollingPath(acme), root, obj{"interval_secs": 1800}).status, 200)
	if s := secondsUntilNextRun(t, app, conn); s > 1 {
		t.Fatalf("a changed setting must queue the connection, next run in %v s", s)
	}
	app.runDue()
	eq(t, runCount(t, app, root, acme, conn), 2)
	if s := secondsUntilNextRun(t, app, conn); s < 1700 || s > 1800 {
		t.Fatalf("next run in %v s, want about 1800", s)
	}
	eq(t, app.get(connectionPath(acme, conn), root).json(t)["effective_interval_secs"], any(float64(1800)))

	r := app.send("PATCH", connectionPath(acme, conn), root, obj{"interval_secs": 600})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["interval_secs"], any(float64(600)))
	eq(t, r.json(t)["effective_interval_secs"], any(float64(600)))
	r = app.send("PATCH", connectionPath(acme, conn), root, obj{"interval_secs": nil})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["interval_secs"], any(nil))
	eq(t, r.json(t)["effective_interval_secs"], any(float64(1800)))
}

func TestPausedConnectionIsNotPolledButRunsByHand(t *testing.T) {
	t.Parallel()
	app := startRepoSyncApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := hostingfake.Start(t, "github", hostingToken, "acme-inc")
	f.Put(hostingfake.Repo{ID: 1, Path: "acme-inc/api"})
	conn := idOf(app.connect(root, acme, f, nil))
	app.runDue()
	secret, _ := app.manualHook(root, acme, conn)

	eq(t, app.send("PUT", pollingPath(acme), root, obj{"mode": "off"}).status, 200)
	eq(t, app.get(connectionPath(acme, conn), root).json(t)["paused"], any(true))
	listed := f.Calls("list")
	app.runDue()
	eq(t, runCount(t, app, root, acme, conn), 1)
	eq(t, f.Calls("list"), listed)
	if s := secondsUntilNextRun(t, app, conn); s < 60 {
		t.Fatalf("a paused connection must be postponed, next run in %v s", s)
	}

	body := `{"ref":"refs/heads/main"}`
	r := request(t, app.addr, "POST", "/api/v1/repo-sync/hooks/"+conn, body, "content-type", "application/json",
		"X-Hub-Signature-256", "sha256="+hmacHex(secret, body))
	eq(t, r.status, 202, r.text())
	eq(t, scalar[string](t, app.db, "SELECT next_trigger FROM repo_sync_connections WHERE id = $1", conn), "schedule")
	app.runDue()
	eq(t, runCount(t, app, root, acme, conn), 1)

	run := app.syncNow(root, acme, conn)
	eq(t, run["trigger"], any("manual"))
	eq(t, run["status"], any("succeeded"))
	eq(t, runCount(t, app, root, acme, conn), 2)

	eq(t, app.send("PUT", pollingPath(acme), root, obj{}).status, 200)
	app.runDue()
	eq(t, runCount(t, app, root, acme, conn), 3)
}

func TestHostingRequestTimeoutComesFromPollingSettings(t *testing.T) {
	t.Parallel()
	app := startRepoSyncApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := hostingfake.Start(t, "github", hostingToken, "acme-inc")
	f.Put(hostingfake.Repo{ID: 1, Path: "acme-inc/api"})
	conn := idOf(app.connect(root, acme, f, nil))
	eq(t, app.send("PUT", pollingPath(acme), root, obj{"timeout_secs": 1}).status, 200)
	f.Delay(2 * time.Second)
	run := app.syncNow(root, acme, conn)
	eq(t, run["status"], any("failed"))
	eq(t, run["error_code"], any("hosting.upstream_error"))

	f.Delay(0)
	eq(t, app.syncNow(root, acme, conn)["status"], any("succeeded"))
}
