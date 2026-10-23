package tests

import (
	"context"
	"testing"
	"time"

	svcapp "svc-registry/internal/app"
	deploytypes "svc-registry/internal/feature/deploy/types"
	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/testsupport/k8sfake"
	"svc-registry/pkg/testing/forgefake"
)

func (n *notifyTree) events(typ string) int64 {
	n.t.Helper()
	return scalar[int64](n.t, n.db, "SELECT count(*) FROM notification_events WHERE type = $1", typ)
}

func (n *notifyTree) deliveriesFor(typ, sub string) int64 {
	n.t.Helper()
	return scalar[int64](n.t, n.db, `SELECT count(*) FROM notification_deliveries d
		JOIN notification_subscriptions s ON s.id = d.subscription_id
		JOIN notification_events e ON e.id = d.event_id WHERE e.type = $1 AND s.key = $2`, typ, sub)
}

func (n *notifyTree) recoversLast(failure, recovery string) {
	n.t.Helper()
	eq(n.t, scalar[string](n.t, n.db, `SELECT payload->>'recovers' FROM notification_events WHERE type = $1
		ORDER BY created_at DESC, id DESC LIMIT 1`, recovery),
		scalar[string](n.t, n.db, `SELECT id::text FROM notification_events WHERE type = $1
		ORDER BY created_at DESC, id DESC LIMIT 1`, failure))
}

func TestDriftResolvedClosesTheEpisode(t *testing.T) {
	t.Parallel()
	k8s := k8sfake.New(saToken)
	n := startNotifyTree(t, []svcapp.Option{svcapp.WithK8s(k8s)}, "NOTIFY_DRIFT_AFTER_SECS", "60")
	n.mustSubscribe(n.root, n.acme, "drift", n.hook(n.root, n.acme, "https://hooks.example/x", "deployment.drift",
		"deployment.drift_resolved"))
	n.mustSubscribe(n.root, n.acme, "plain", n.hook(n.root, n.acme, "https://hooks.example/y", "deployment.drift"))
	n.cluster(n.root, "prod", nil)
	ann := map[string]string{deploytypes.AnnotationProject: "acme/backend/api", deploytypes.AnnotationService: "api"}
	drift := func(version string) {
		t.Helper()
		k8s.Cluster().Put(workload("u1", "api", version, ann))
		n.pollNow()
		execSQL(t, n.db, "UPDATE cluster_workloads SET version_since = now() - interval '10 minutes'")
		n.pollNow()
	}
	eq(t, n.ingest(n.api, n.apiKey, deployed("d1", "api", "production", "1.5.0", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))).status, 201)
	drift("1.4.3")
	eq(t, n.events("deployment.drift"), int64(1))
	eq(t, n.events("deployment.drift_resolved"), int64(0))
	n.mustSubscribe(n.root, n.backend, "late", n.hook(n.root, n.backend, "https://hooks.example/z", "deployment.drift",
		"deployment.drift_resolved"))

	k8s.Cluster().Put(workload("u1", "api", "1.5.0", ann))
	n.pollNow()
	eq(t, n.events("deployment.drift_resolved"), int64(1))
	eq(t, n.deliveriesFor("deployment.drift_resolved", "drift"), int64(1))
	eq(t, n.deliveriesFor("deployment.drift_resolved", "plain"), int64(0), "not subscribed to the recovery")
	eq(t, n.deliveriesFor("deployment.drift_resolved", "late"), int64(0), "got no failure of the episode")
	n.recoversLast("deployment.drift", "deployment.drift_resolved")
	eq(t, scalar[string](t, n.db, "SELECT (payload->'data'->>'version') || ' ' || (payload->'data'->>'environment') FROM notification_events WHERE type = 'deployment.drift_resolved'"),
		"1.5.0 production")
	n.pollNow()
	eq(t, n.events("deployment.drift_resolved"), int64(1), "the episode is closed")
	eq(t, n.count("notification_episodes"), int64(0))

	drift("1.4.3")
	eq(t, n.events("deployment.drift"), int64(2), "the same state after a recovery is a new episode")
}

func TestLinkRecoveryNeedsANotifiedFailure(t *testing.T) {
	t.Parallel()
	n := startNotifyTree(t, nil)
	n.mustSubscribe(n.root, n.acme, "links", n.hook(n.root, n.acme, "https://hooks.example/x", "deployment.created"))
	n.putTemplate(n.root, n.backend, "grafana", "https://grafana.example/d/{project.slug}")
	n.links(n.root, n.api, "")
	checker := &scriptedChecker{status: linkstypes.StatusOK}
	n.checker.set(checker)
	check := func(status linkstypes.Status) {
		t.Helper()
		checker.status = status
		execSQL(t, n.db, "UPDATE link_targets SET next_run_at = now(), lease_until = NULL")
		ctx := context.Background()
		targets, err := n.services.Links.Checks.ClaimLinkChecks(ctx, 100, 60)
		must(t, err)
		for _, tg := range targets {
			must(t, n.services.Links.Checks.RunLinkCheck(ctx, tg))
		}
	}
	check(linkstypes.StatusOK)
	check(linkstypes.StatusServerError)
	check(linkstypes.StatusOK)
	eq(t, n.count("notification_events"), int64(0), "no failure was notified")

	n.mustSubscribe(n.root, n.acme, "links", n.hook(n.root, n.acme, "https://hooks.example/x", "link.check_failed",
		"link.check_recovered"))
	check(linkstypes.StatusServerError)
	check(linkstypes.StatusTimeout)
	eq(t, n.events("link.check_failed"), int64(1))
	check(linkstypes.StatusOK)
	eq(t, n.events("link.check_recovered"), int64(1))
	n.recoversLast("link.check_failed", "link.check_recovered")
	eq(t, scalar[string](t, n.db, "SELECT (payload->'data'->>'status') || ' ' || (payload->'data'->>'previous_status') FROM notification_events WHERE type = 'link.check_recovered'"),
		"ok timeout")
	check(linkstypes.StatusOK)
	eq(t, n.events("link.check_recovered"), int64(1))
	check(linkstypes.StatusServerError)
	eq(t, n.events("link.check_failed"), int64(2))
}

func TestForgeSyncRecoveryAfterAFailure(t *testing.T) {
	t.Parallel()
	n := startNotifyTree(t, nil)
	n.mustSubscribe(n.root, n.acme, "forge", n.hook(n.root, n.acme, "https://hooks.example/x", "forge.sync_failed",
		"forge.sync_recovered"))
	f := forgefake.Start(t, "gitlab", forgeToken, "acme")
	secret := n.secret(n.root, n.acme, obj{"value": forgeToken})
	conn := idOf(n.connect(n.root, n.acme, f, obj{"credentials": obj{"secret_id": secret}}))
	eq(t, n.syncNow(n.root, n.acme, conn)["status"], any("succeeded"))
	eq(t, n.count("notification_events"), int64(0), "success without a failure")
	setToken := func(v string) {
		t.Helper()
		eq(t, n.send("PATCH", secretsPath(n.acme)+"/"+secret, n.root, obj{"value": v}).status, 200)
	}
	setToken("revoked")
	eq(t, n.syncNow(n.root, n.acme, conn)["status"], any("failed"))
	setToken(forgeToken)
	eq(t, n.syncNow(n.root, n.acme, conn)["status"], any("succeeded"))
	eq(t, n.events("forge.sync_recovered"), int64(1))
	n.recoversLast("forge.sync_failed", "forge.sync_recovered")
	eq(t, scalar[string](t, n.db, "SELECT (payload->'data'->>'forge') || ' ' || coalesce(payload->'data'->>'error_code', 'none') FROM notification_events WHERE type = 'forge.sync_recovered'"),
		"gitlab none")
	eq(t, n.syncNow(n.root, n.acme, conn)["status"], any("succeeded"))
	eq(t, n.events("forge.sync_recovered"), int64(1))
	setToken("revoked")
	eq(t, n.syncNow(n.root, n.acme, conn)["status"], any("failed"))
	eq(t, n.events("forge.sync_failed"), int64(2))
}
