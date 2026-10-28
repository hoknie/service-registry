package tests

import (
	"context"
	"testing"
	"time"

	"svc-registry/internal/presentation/jobs"
	"svc-registry/pkg/testing/hostingfake"
)

func init() { jobs.RepoSyncTick = 50 * time.Millisecond }

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
	app := startRepoSyncApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := hostingfake.Start(t, "github", hostingToken, "acme-inc")
	f.Put(hostingfake.Repo{ID: 1, Path: "acme-inc/api"})
	app.connect(root, acme, f, nil)
	second := appOver(t, app.db, "SECRETS_KEYS", secretsKey)

	ctx, cancel := context.WithCancel(context.Background())
	done1 := jobs.SpawnRepoSync(ctx, app.services.Jobs())
	done2 := jobs.SpawnRepoSync(ctx, second.services.Jobs())
	waitFor(t, "a run", func() bool { return app.count("repo_sync_runs") > 0 })
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done1
	<-done2
	eq(t, app.count("repo_sync_runs"), int64(1))
	eq(t, f.Calls("list"), 1)
	eq(t, scalar[string](t, app.db, "SELECT trigger FROM repo_sync_runs"), "schedule")
	eq(t, scalar[bool](t, app.db, "SELECT lease_until IS NULL AND next_run_at > now() + interval '10 minutes' FROM repo_sync_connections"), true)
}

func TestJobsCanBeDisabled(t *testing.T) {
	t.Parallel()
	app := startRepoSyncApp(t, "BACKGROUND_JOBS_ENABLED", "false")
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	app.connect(root, acme, hostingfake.Start(t, "github", hostingToken, "acme-inc"), nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := jobs.SpawnRepoSync(ctx, app.services.Jobs())
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
	eq(t, app.count("repo_sync_runs"), int64(0))
}

func TestShutdownInterruptsARunningSync(t *testing.T) {
	t.Parallel()
	app := startRepoSyncApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := hostingfake.Start(t, "github", hostingToken, "acme-inc")
	f.Put(hostingfake.Repo{ID: 1, Path: "acme-inc/api"})
	f.Delay(5 * time.Second)
	app.connect(root, acme, f, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := jobs.SpawnRepoSync(ctx, app.services.Jobs())
	waitFor(t, "a running run", func() bool { return app.count("repo_sync_runs") > 0 })
	cancel()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("scheduler did not stop")
	}
	eq(t, scalar[string](t, app.db, "SELECT status FROM repo_sync_runs"), "failed")
	eq(t, scalar[string](t, app.db, "SELECT error_code FROM repo_sync_runs"), "repo_sync.interrupted")
	eq(t, scalar[bool](t, app.db, "SELECT lease_until IS NULL AND next_run_at <= now() FROM repo_sync_connections"), true, "due again at once")
}
