package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	svcapp "svc-registry/internal/app"
	deploytypes "svc-registry/internal/feature/deploy/types"
	linkstypes "svc-registry/internal/feature/links/types"
	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/internal/testsupport/k8sfake"
	"svc-registry/pkg/testing/forgefake"
)

type notifyTree struct {
	*testApp
	root, acme, backend, sandbox, api, apiKey, tool, toolKey string
}

func startNotifyTree(t *testing.T, opts []svcapp.Option, extra ...string) *notifyTree {
	t.Helper()
	app := startNotifyApp(t, opts, extra...)
	n := &notifyTree{testApp: app, root: app.admin()}
	n.acme = app.nodeID(n.root, "organization", "", "acme")
	n.backend = app.nodeID(n.root, "folder", n.acme, "backend")
	n.sandbox = app.nodeID(n.root, "folder", n.acme, "sandbox")
	api := app.node(n.root, "project", n.backend, "api")
	n.api, n.apiKey = idOf(api), at(api, "key", "secret").(string)
	tool := app.node(n.root, "project", n.sandbox, "tool")
	n.tool, n.toolKey = idOf(tool), at(tool, "key", "secret").(string)
	return n
}

func (n *notifyTree) deliveries(sub string) int64 {
	n.t.Helper()
	return scalar[int64](n.t, n.db, `SELECT count(*) FROM notification_deliveries d
		JOIN notification_subscriptions s ON s.id = d.subscription_id WHERE s.key = $1`, sub)
}

func TestDeploymentNotificationsReachSubscriptionsAbove(t *testing.T) {
	t.Parallel()
	n := startNotifyTree(t, nil)
	rc := newReceiver(t)
	n.mustSubscribe(n.root, n.acme, "ops", n.hook(n.root, n.acme, rc.URL))
	prod := n.hook(n.root, n.acme, rc.URL)
	prod["environments"] = []string{"production"}
	n.mustSubscribe(n.root, n.acme, "prod", prod)
	n.mustSubscribe(n.root, n.sandbox, "ops", obj{"channel": "webhook", "event_types": []string{"deployment.created"}, "disabled": true})
	g := createGroup(t, n.testApp, n.root, "Platform")
	eq(t, n.send("PUT", "/api/v1/groups/"+idOf(g)+"/contacts", n.root, obj{"contacts": []obj{},
		"on_call_url": "https://oncall.example/platform"}).status, 200)
	eq(t, n.send("PUT", ownersPath(n.api), n.root, obj{"owners": []obj{{"subject_kind": "group", "subject": "Platform"}}}).status, 200)

	body := deployed("d1", "api", "production", "2.0.0", time.Now().Add(-time.Minute).UTC().Format(time.RFC3339))
	body["payload"].(obj)["commit_sha"] = "abcdef1"
	eq(t, n.ingest(n.api, n.apiKey, body).status, 201)
	eq(t, n.deliveries("ops"), int64(1))
	eq(t, n.deliveries("prod"), int64(1))
	eq(t, n.ingest(n.api, n.apiKey, body).status, 200)
	eq(t, n.count("notification_deliveries"), int64(2), "a replay notifies nobody")

	eq(t, n.ingest(n.api, n.apiKey, deployed("d2", "api", "staging", "2.0.1", time.Now().UTC().Format(time.RFC3339))).status, 201)
	eq(t, n.deliveries("ops"), int64(2))
	eq(t, n.deliveries("prod"), int64(1), "the environment filter")
	eq(t, n.ingest(n.tool, n.toolKey, deployed("t1", "tool", "production", "1.0.0", time.Now().UTC().Format(time.RFC3339))).status, 201)
	eq(t, n.deliveries("ops"), int64(2), "disabled in sandbox")
	eq(t, n.deliveries("prod"), int64(2), "prod is not disabled in sandbox")

	eq(t, n.deliverNow(), 4)
	ids := map[any]int{}
	var p obj
	for i := range rc.requests() {
		if v := rc.payload(t, i); at(v, "data", "version") == "2.0.0" {
			p = v
			ids[v["id"]]++
		}
	}
	eq(t, len(ids), 1, "one event, one id")
	eq(t, ids[p["id"]], 2)
	eq(t, p["type"], any("deployment.created"))
	eq(t, at(p, "node", "path"), any("acme/backend/api"))
	eq(t, at(p, "data", "service"), any("api"))
	eq(t, at(p, "data", "environment"), any("production"))
	eq(t, at(p, "data", "version"), any("2.0.0"))
	eq(t, at(p, "data", "commit_sha"), any("abcdef1"))
	eq(t, at(p, "data", "source"), any("event"))
	eq(t, at(p, "data", "current"), any(true))
	eq(t, at(p, "owners", 0, "name"), any("Platform"))
	eq(t, at(p, "owners", 0, "on_call_url"), any("https://oncall.example/platform"))
	eq(t, has(at(p, "owners", 0).(obj), "node_id"), false)
}

func TestRolledBackFactLeavesNoNotification(t *testing.T) {
	t.Parallel()
	n := startNotifyTree(t, nil)
	n.mustSubscribe(n.root, n.acme, "ops", n.hook(n.root, n.acme, "https://hooks.example/x"))
	project := uuid.MustParse(n.api)
	errBoom := errors.New("boom")
	err := n.services.DB.InTx(context.Background(), func(ctx context.Context) error {
		if err := n.services.Notify.Outbox.Emit(ctx, notifytypes.Event{Type: notifytypes.EventDeploymentCreated, NodeID: project,
			DedupeKey: "dep:" + uuid.NewString(), OccurredAt: time.Now(), Data: notifytypes.DeploymentData{Service: "api"}}); err != nil {
			return err
		}
		eq(t, scalar[int64](t, n.db, "SELECT count(*) FROM notification_deliveries"), int64(0), "not visible before commit")
		return errBoom
	})
	eq(t, errors.Is(err, errBoom), true)
	eq(t, n.count("notification_events"), int64(0))
	eq(t, n.count("notification_deliveries"), int64(0))

	must(t, n.services.Notify.Outbox.Emit(context.Background(), notifytypes.Event{Type: notifytypes.EventDeploymentCreated,
		NodeID: project, DedupeKey: "dep:x", OccurredAt: time.Now(), Data: notifytypes.DeploymentData{Service: "api"}}))
	event := scalar[string](t, n.db, "SELECT id::text FROM notification_events")
	sub := scalar[string](t, n.db, "SELECT id::text FROM notification_subscriptions")
	contains(t, execErr(t, n.db, "INSERT INTO notification_deliveries (id, event_id, subscription_id) VALUES ($1, $2, $3)",
		uuid.New(), event, sub), "notification_deliveries_event_subscription_key")
	contains(t, execErr(t, n.db, "DELETE FROM secrets"), "notification_subscriptions_secret_id_fkey")
}

func TestDriftNotifiesOncePerEpisode(t *testing.T) {
	t.Parallel()
	k8s := k8sfake.New(saToken)
	n := startNotifyTree(t, []svcapp.Option{svcapp.WithK8s(k8s)}, "NOTIFY_DRIFT_AFTER_SECS", "60")
	n.mustSubscribe(n.root, n.acme, "drift", n.hook(n.root, n.acme, "https://hooks.example/x", "deployment.drift", "deployment.created"))
	n.cluster(n.root, "prod", nil)
	ann := map[string]string{deploytypes.AnnotationProject: "acme/backend/api", deploytypes.AnnotationService: "api"}
	k8s.Cluster().Put(workload("u1", "api", "1.4.3", ann))
	eq(t, n.ingest(n.api, n.apiKey, deployed("d1", "api", "production", "1.4.3", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))).status, 201)
	n.pollNow()
	drifts := func() int64 {
		return scalar[int64](t, n.db, "SELECT count(*) FROM notification_events WHERE type = 'deployment.drift'")
	}
	eq(t, drifts(), int64(0), "versions agree")

	eq(t, n.ingest(n.api, n.apiKey, deployed("d2", "api", "production", "1.5.0", time.Now().Add(-10*time.Second).UTC().Format(time.RFC3339))).status, 201)
	n.pollNow()
	eq(t, drifts(), int64(0), "the drift is younger than the threshold")
	execSQL(t, n.db, "UPDATE service_environments SET occurred_at = now() - interval '10 minutes'")
	n.pollNow()
	eq(t, drifts(), int64(0), "the cluster version is observed for less than the threshold")
	execSQL(t, n.db, "UPDATE cluster_workloads SET version_since = now() - interval '10 minutes'")
	n.pollNow()
	eq(t, drifts(), int64(1))
	n.pollNow()
	eq(t, drifts(), int64(1), "the same episode")
	data := scalar[string](t, n.db, "SELECT payload->'data'->>'version' || ' ' || (payload->'data'->'observed'->0->>'version') FROM notification_events WHERE type = 'deployment.drift'")
	eq(t, data, "1.5.0 1.4.3")

	k8s.Cluster().Put(workload("u1", "api", "1.4.4", ann))
	n.pollNow()
	eq(t, drifts(), int64(1), "a new observed version starts a new episode, younger than the threshold")
	execSQL(t, n.db, "UPDATE cluster_workloads SET version_since = now() - interval '10 minutes'")
	n.pollNow()
	eq(t, drifts(), int64(2))
}

func TestClusterDeploymentNotifies(t *testing.T) {
	t.Parallel()
	k8s := k8sfake.New(saToken)
	n := startNotifyTree(t, []svcapp.Option{svcapp.WithK8s(k8s)}, "K8S_HISTORY_CONFIRM_SECS", "0")
	n.mustSubscribe(n.root, n.acme, "ops", n.hook(n.root, n.acme, "https://hooks.example/x"))
	n.cluster(n.root, "prod", nil)
	k8s.Cluster().Put(workload("u1", "api", "1.4.3", map[string]string{deploytypes.AnnotationProject: "acme/backend/api",
		deploytypes.AnnotationService: "api"}))
	n.pollNow()
	n.pollNow()
	eq(t, scalar[string](t, n.db, "SELECT payload->'data'->>'source' || ' ' || (payload->'data'->>'cluster') || ' ' || (payload->'data'->>'namespace') || ' ' || (payload->'data'->>'version') FROM notification_events"),
		"cluster prod backend 1.4.3")
}

type scriptedChecker struct{ status linkstypes.Status }

func (s *scriptedChecker) Check(context.Context, string) linkstypes.Result {
	return linkstypes.Result{Status: s.status}
}

func TestBrokenLinkNotifiesOnTransition(t *testing.T) {
	t.Parallel()
	n := startNotifyTree(t, nil)
	n.mustSubscribe(n.root, n.acme, "links", n.hook(n.root, n.acme, "https://hooks.example/x", "link.check_failed"))
	n.putTemplate(n.root, n.backend, "grafana", "https://grafana.example/d/{project.slug}")
	n.links(n.root, n.api, "")
	n.links(n.root, n.tool, "")
	checker := &scriptedChecker{status: linkstypes.StatusNotFound}
	n.checker.set(checker)
	failed := func() int64 {
		return scalar[int64](t, n.db, "SELECT count(*) FROM notification_events WHERE type = 'link.check_failed'")
	}
	check := func() {
		t.Helper()
		execSQL(t, n.db, "UPDATE link_targets SET next_run_at = now(), lease_until = NULL")
		ctx := context.Background()
		targets, err := n.services.Links.Checks.ClaimLinkChecks(ctx, 100, 60)
		must(t, err)
		for _, tg := range targets {
			must(t, n.services.Links.Checks.RunLinkCheck(ctx, tg))
		}
	}
	check()
	eq(t, failed(), int64(0), "a link that never worked")
	checker.status = linkstypes.StatusOK
	check()
	checker.status = linkstypes.StatusServerError
	check()
	eq(t, failed(), int64(1))
	check()
	eq(t, failed(), int64(1), "still broken")
	eq(t, scalar[string](t, n.db, "SELECT (payload->'data'->>'status') || ' ' || (payload->'data'->>'previous_status') || ' ' || (payload->'data'->>'link_key') || ' ' || (payload->'data'->>'kind_key') || ' ' || (payload->'node'->>'path') FROM notification_events"),
		"server_error ok grafana grafana acme/backend/api")
	checker.status = linkstypes.StatusBlocked
	check()
	checker.status = linkstypes.StatusTimeout
	check()
	eq(t, failed(), int64(1), "blocked is not working")
}

func TestForgeSyncFailureNotifiesOnce(t *testing.T) {
	t.Parallel()
	n := startNotifyTree(t, nil)
	n.mustSubscribe(n.root, n.acme, "forge", n.hook(n.root, n.acme, "https://hooks.example/x", "forge.sync_failed"))
	f := forgefake.Start(t, "gitlab", forgeToken, "acme")
	secret := n.secret(n.root, n.acme, obj{"value": forgeToken})
	conn := idOf(n.connect(n.root, n.acme, f, obj{"credentials": obj{"secret_id": secret}}))
	eq(t, n.syncNow(n.root, n.acme, conn)["status"], any("succeeded"))
	eq(t, n.send("PATCH", secretsPath(n.acme)+"/"+secret, n.root, obj{"value": "revoked"}).status, 200)
	eq(t, n.syncNow(n.root, n.acme, conn)["status"], any("failed"))
	eq(t, n.syncNow(n.root, n.acme, conn)["status"], any("failed"))
	eq(t, n.count("notification_events"), int64(1))
	eq(t, scalar[string](t, n.db, "SELECT (payload->'data'->>'error_code') || ' ' || (payload->'node'->>'path') FROM notification_events"),
		"forge.unauthorized acme")
}
