package tests

import (
	"context"
	"testing"
	"time"

	svcapp "svc-registry/internal/app"
	"svc-registry/internal/deploy"
	"svc-registry/internal/service"
)

func init() { svcapp.ClusterTick = 50 * time.Millisecond }

func (a *testApp) pollNow() {
	a.t.Helper()
	execSQL(a.t, a.db, "UPDATE clusters SET next_run_at = now()")
	ctx := context.Background()
	ids, err := service.ClaimClusterPolls(ctx, a.state, 100, 120)
	if err != nil {
		a.t.Fatal(err)
	}
	for _, id := range ids {
		if err := service.RunClusterPoll(ctx, a.state, id); err != nil {
			a.t.Fatal(err)
		}
	}
}

func workload(uid, name, image string, ann map[string]string) deploy.Workload {
	return deploy.Workload{UID: uid, Kind: deploy.KindDeployment, Namespace: "backend", Name: name, Annotations: ann,
		Labels: map[string]string{"app.kubernetes.io/name": name}, Selector: "app=" + name,
		Containers: []deploy.Container{{Name: "app", Image: "registry.example/acme/" + name + ":" + image}},
		Generation: 1, ObservedGeneration: 1, Desired: 2, Ready: 2, Updated: 2}
}

func TestPollMatchesAndObservesWorkloads(t *testing.T) {
	t.Parallel()
	app, k8s := startClusterApp(t)
	root := app.admin()
	org := app.nodeID(root, "organization", "", "acme")
	folder := app.nodeID(root, "folder", org, "backend")
	api := app.nodeID(root, "project", folder, "api")
	off := app.node(root, "project", folder, "legacy")
	eq(t, app.send("PATCH", nodePath(idOf(off)), root, obj{"cluster_observation": false}).status, 200)
	c := app.cluster(root, "prod", obj{"rules": []obj{{"label": "app.kubernetes.io/name", "project": "acme/{namespace}/{label:app.kubernetes.io/name}"}}})

	byPath := workload("u1", "billing", "1.4.2", map[string]string{deploy.AnnotationProject: "acme/backend/api",
		deploy.AnnotationBranch: "release/1.4"})
	byTemplate := workload("u2", "worker", "2.0.0", nil)
	byTemplate.TemplateAnnotations = map[string]string{deploy.AnnotationProject: api, deploy.AnnotationEnvironment: "staging"}
	byRule := workload("u3", "api", "1.0.0", nil)
	noAnnotation := workload("u4", "redis", "7", nil)
	noAnnotation.Labels = nil
	unknown := workload("u5", "ghost", "1", map[string]string{deploy.AnnotationProject: "acme/nope"})
	legacy := workload("u6", "legacy", "1", map[string]string{deploy.AnnotationProject: "acme/backend/legacy"})
	cron := deploy.Workload{UID: "u7", Kind: deploy.KindCronJob, Namespace: "backend", Name: "report", Schedule: "0 * * * *",
		Annotations: map[string]string{deploy.AnnotationProject: "acme/backend/api"}, Containers: []deploy.Container{{Name: "job", Image: "report:3"}}}
	k8s.Cluster().Put(byPath, byTemplate, byRule, noAnnotation, unknown, legacy, cron)
	k8s.Cluster().PutPods("backend", "app=billing", deploy.Pod{Containers: []deploy.PodContainer{{Name: "app", ImageID: "x@sha256:aa", Restarts: 3}}})
	failed := time.Now().Add(-time.Hour)
	k8s.Cluster().PutJobs("u7", deploy.Job{StartedAt: &failed, Failed: 1})
	app.pollNow()

	got := app.get(clusterPath(idOf(c)), root).json(t)
	eq(t, got["status"], any("ok"))
	eq(t, got["workloads"], any(float64(4)))
	eq(t, got["unmatched"], any(float64(2)))
	eq(t, scalar[string](t, app.db, "SELECT service || '/' || environment FROM cluster_workloads WHERE uid = 'u1'"), "billing/production")
	eq(t, scalar[string](t, app.db, "SELECT environment FROM cluster_workloads WHERE uid = 'u2'"), "staging")
	eq(t, scalar[string](t, app.db, "SELECT state->>'restarts' FROM cluster_workloads WHERE uid = 'u1'"), "3")
	eq(t, scalar[string](t, app.db, "SELECT images->0->>'digest' FROM cluster_workloads WHERE uid = 'u1'"), "sha256:aa")
	eq(t, scalar[string](t, app.db, "SELECT state->'last_run'->>'status' FROM cluster_workloads WHERE uid = 'u7'"), "failed")
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM cluster_workloads WHERE uid = 'u6'"), int64(0), "observation off")
	branch := app.get(nodePath(api)+"/branches/item?name=release%2F1.4", root)
	eq(t, branch.status, 200, branch.text())
	eq(t, jsonText(branch.json(t)["sources"]), `["cluster"]`)

	r := app.get(clusterPath(idOf(c))+"/unmatched", root)
	eq(t, r.status, 200, r.text())
	items := list(t, r.json(t), "items")
	eq(t, len(items), 2)
	eq(t, items[0].(obj)["name"], any("ghost"))
	eq(t, items[0].(obj)["reason"], any("project_not_found"))
	eq(t, items[0].(obj)["annotation"], any("acme/nope"))
	eq(t, items[1].(obj)["reason"], any("no_annotation"))

	k8s.Cluster().Put(byPath, byTemplate, byRule, noAnnotation, unknown, legacy)
	app.pollNow()
	eq(t, scalar[bool](t, app.db, "SELECT gone_at IS NOT NULL FROM cluster_workloads WHERE uid = 'u7'"), true)
	k8s.Cluster().Fail(&deploy.Failure{Code: deploy.FailUnreachable, Message: "dial tcp: refused"})
	app.pollNow()
	got = app.get(clusterPath(idOf(c)), root).json(t)
	eq(t, got["status"], any("error"))
	eq(t, at(got, "last_error", "code"), any("k8s.unreachable"))
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM cluster_workloads WHERE gone_at IS NULL"), int64(5))

	k8s.Cluster().Fail(nil)
	eq(t, app.send("PATCH", nodePath(api), root, obj{"cluster_observation": false}).status, 200)
	app.pollNow()
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM cluster_workloads WHERE project_id = $1", api), int64(0))
	eq(t, len(list(t, app.get(clusterPath(idOf(c))+"/unmatched", root).json(t), "items")), 2)
}

func TestDeploymentsFromClusters(t *testing.T) {
	t.Parallel()
	app, k8s := startClusterApp(t)
	root := app.admin()
	project, key := app.projectWithKey(root, "api")
	c := app.cluster(root, "prod", nil)
	ann := map[string]string{deploy.AnnotationProject: "api-org/api", deploy.AnnotationService: "api"}
	eq(t, app.ingest(project, key, deployed("k1", "api", "production", "1.4.2", "2026-10-01T10:00:00Z")).status, 201)

	k8s.Cluster().Put(workload("u1", "api", "1.4.3", ann))
	app.pollNow()
	eq(t, scalar[string](t, app.db, "SELECT pending_version FROM cluster_workloads"), "1.4.3")
	eq(t, app.count("service_deployments"), int64(1))
	app.pollNow()
	eq(t, app.count("service_deployments"), int64(1), "still within K8S_HISTORY_CONFIRM_SECS")
	execSQL(t, app.db, "UPDATE cluster_workloads SET pending_since = pending_since - interval '11 minutes'")
	app.pollNow()
	eq(t, app.count("service_deployments"), int64(2))
	r := app.get(nodePath(project)+"/deployments", root)
	first := list(t, r.json(t), "items")[0].(obj)
	eq(t, first["version"], any("1.4.3"))
	eq(t, first["source"], any("cluster"))
	eq(t, first["event_id"], nil)
	eq(t, first["deployed_by"], nil)
	eq(t, first["cluster"], any("prod"))
	eq(t, first["current"], any(true))
	app.pollNow()
	eq(t, app.count("service_deployments"), int64(2), "one change, one deployment")
	eq(t, scalar[*string](t, app.db, "SELECT pending_version FROM cluster_workloads") == nil, true)

	k8s.Cluster().Put(workload("u1", "api", "1.5.0", ann))
	app.pollNow()
	eq(t, app.ingest(project, key, deployed("k2", "api", "production", "1.5.0", time.Now().UTC().Format(time.RFC3339))).status, 201)
	execSQL(t, app.db, "UPDATE cluster_workloads SET pending_since = pending_since - interval '11 minutes'")
	app.pollNow()
	eq(t, app.count("service_deployments"), int64(3))
	eq(t, scalar[string](t, app.db, "SELECT source FROM service_deployments WHERE version = '1.5.0'"), "event")

	eq(t, app.call("DELETE", clusterPath(idOf(c)), root).status, 204)
	eq(t, app.count("cluster_workloads"), int64(0))
	eq(t, app.count("service_deployments"), int64(3))
}

func TestTwoSchedulersPollAClusterOnce(t *testing.T) {
	t.Parallel()
	app, k8s := startClusterApp(t)
	root := app.admin()
	app.cluster(root, "prod", nil)
	second := appOver(t, app.db, "SECRETS_KEYS", secretsKey)
	second.state.K8s = k8s
	ctx, cancel := context.WithCancel(context.Background())
	done1 := svcapp.SpawnClusterPolls(ctx, app.state)
	done2 := svcapp.SpawnClusterPolls(ctx, second.state)
	waitFor(t, "a poll", func() bool { return k8s.Cluster().Calls() > 0 })
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done1
	<-done2
	eq(t, k8s.Cluster().Calls(), 1)
	eq(t, scalar[string](t, app.db, "SELECT status FROM clusters"), "ok")
}

func TestClusterPollsStopWithBackgroundJobs(t *testing.T) {
	t.Parallel()
	app, k8s := startClusterApp(t, "BACKGROUND_JOBS_ENABLED", "false")
	root := app.admin()
	app.cluster(root, "prod", nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := svcapp.SpawnClusterPolls(ctx, app.state)
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
	eq(t, k8s.Cluster().Calls(), 0)
	eq(t, list(t, app.get(clustersPath, root).json(t), "items")[0].(obj)["status"], any("never"))
}
