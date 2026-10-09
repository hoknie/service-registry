package tests

import (
	"context"
	"testing"
	"time"

	svcapp "svc-registry/internal/app"
	"svc-registry/internal/testsupport/forgefake"
)

func init() { svcapp.ForgeTick = 50 * time.Millisecond }

func waitFor(t testing.TB, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestTwoSchedulersRunAConnectionOnce(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api"})
	app.connect(root, acme, f, nil)
	second := appOver(t, app.db, "SECRETS_KEYS", secretsKey)

	ctx, cancel := context.WithCancel(context.Background())
	done1 := svcapp.SpawnForgeSync(ctx, app.state)
	done2 := svcapp.SpawnForgeSync(ctx, second.state)
	waitFor(t, "a run", func() bool { return app.count("forge_sync_runs") > 0 })
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done1
	<-done2
	eq(t, app.count("forge_sync_runs"), int64(1))
	eq(t, f.Calls("list"), 1)
	eq(t, scalar[string](t, app.db, "SELECT trigger FROM forge_sync_runs"), "schedule")
	eq(t, scalar[bool](t, app.db, "SELECT lease_until IS NULL AND next_run_at > now() + interval '10 minutes' FROM forge_connections"), true)
}

func TestJobsCanBeDisabled(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t, "BACKGROUND_JOBS_ENABLED", "false")
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	app.connect(root, acme, forgefake.Start(t, "github", forgeToken, "acme-inc"), nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := svcapp.SpawnForgeSync(ctx, app.state)
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
	eq(t, app.count("forge_sync_runs"), int64(0))
}

func TestShutdownInterruptsARunningSync(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api"})
	f.Delay(5 * time.Second)
	app.connect(root, acme, f, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := svcapp.SpawnForgeSync(ctx, app.state)
	waitFor(t, "a running run", func() bool { return app.count("forge_sync_runs") > 0 })
	cancel()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("scheduler did not stop")
	}
	eq(t, scalar[string](t, app.db, "SELECT status FROM forge_sync_runs"), "failed")
	eq(t, scalar[string](t, app.db, "SELECT error_code FROM forge_sync_runs"), "forge.interrupted")
	eq(t, scalar[bool](t, app.db, "SELECT lease_until IS NULL AND next_run_at <= now() FROM forge_connections"), true, "due again at once")
}
