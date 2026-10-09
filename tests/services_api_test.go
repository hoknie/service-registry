package tests

import (
	"fmt"
	"strings"
	"testing"

	"svc-registry/internal/deploy"
)

func recordsPath(project, what string) string { return nodePath(project) + "/" + what }

func versions(t *testing.T, v obj) string {
	t.Helper()
	var out []string
	for _, d := range list(t, v, "items") {
		out = append(out, d.(obj)["version"].(string))
	}
	return strings.Join(out, ",")
}

func TestReadingNeedsASessionAndCatalogReadOnAProject(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")
	org := app.get(nodePath(pid), admin).json(t)["parent_id"].(string)
	folder := app.nodeID(admin, "folder", org, "backend")

	what := []string{"events", "deployments", "environments"}
	for _, w := range what {
		r := request(t, app.addr, "GET", recordsPath(pid, w), "", "authorization", "Bearer "+key)
		eq(t, r.status, 401, w)
		eq(t, r.json(t)["code"], any("auth.invalid_token"))
	}

	bob := bobSession(app)
	for _, w := range what {
		eq(t, app.get(recordsPath(pid, w), bob).status, 404, w, ": invisible")
	}
	app.grant(admin, org, "user", "bob@example.com", "viewer")
	for _, w := range what {
		r := app.get(recordsPath(pid, w), bob)
		eq(t, r.status, 200, w, ": ", r.text())
		r = app.get(recordsPath(folder, w), admin)
		eq(t, r.status, 404, w, ": not a project")
		eq(t, r.json(t)["code"], any("not_found"))
	}
	eq(t, app.get(recordsPath(pid, "environments"), bob).text(), `{"items":[]}`)
	r := app.get(recordsPath(pid, "events")+"?limit=0", bob)
	eq(t, r.status, 400)
	eq(t, r.json(t)["code"], any("validation.invalid_pagination"))
}

func TestListsAreOrderedFilteredAndFlagTheCurrentDeployment(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")
	send := func(k, svc, env, ver, at, branch string) obj {
		e := deployed(k, svc, env, ver, at)
		e["payload"].(obj)["branch"] = branch
		return e
	}
	for _, e := range []obj{
		send("1", "api", "production", "2.0.0", "2025-06-01T10:00:00Z", "main"),
		send("2", "api", "staging", "2.1.0", "2025-06-01T11:00:00Z", "feature/x"),
		send("3", "api", "production", "2.1.0", "2025-06-01T12:00:00Z", "main"),
		send("4", "worker", "production", "0.1.0", "2025-06-01T09:00:00Z", "main"),
	} {
		r := app.ingest(pid, key, e)
		eq(t, r.status, 201, r.text())
	}

	events := app.get(recordsPath(pid, "events"), admin).json(t)
	eq(t, events["total"], any(float64(4)))
	var order []string
	for _, e := range list(t, events, "items") {
		order = append(order, e.(obj)["idempotency_key"].(string))
	}
	eq(t, strings.Join(order, ","), "4,3,2,1", "last received first")
	eq(t, strings.HasPrefix(at(events, "items", 0, "key_prefix").(string), "svcr_"), true)
	eq(t, at(events, "items", 0, "payload", "service"), any("worker"))
	eq(t, app.get(recordsPath(pid, "events")+"?type=build.finished", admin).json(t)["total"], any(float64(0)))

	all := app.get(recordsPath(pid, "deployments"), admin).json(t)
	eq(t, versions(t, all), "2.1.0,2.1.0,2.0.0,0.1.0")
	var current []string
	for _, d := range list(t, all, "items") {
		current = append(current, fmt.Sprintf("%s=%v", d.(obj)["environment"], d.(obj)["current"]))
	}
	eq(t, strings.Join(current, ","), "production=true,staging=true,production=false,production=true")

	prodMain := app.get(recordsPath(pid, "deployments")+"?environment=Production&branch=main", admin).json(t)
	eq(t, prodMain["total"], any(float64(3)))
	feature := app.get(recordsPath(pid, "deployments")+"?branch=feature%2Fx", admin).json(t)
	eq(t, versions(t, feature), "2.1.0")
	eq(t, at(feature, "items", 0, "environment"), any("staging"))
	worker := app.get(recordsPath(pid, "deployments")+"?service=worker&limit=1&offset=0", admin).json(t)
	eq(t, jsonText([]any{worker["total"], worker["limit"]}), "[1,1]")
	plus := app.get(recordsPath(pid, "deployments")+"?service=+Worker+", admin).json(t)
	eq(t, plus["total"], any(float64(1)), "+ is a space, trimmed and lower-cased like on ingest")
	page2 := app.get(recordsPath(pid, "deployments")+"?limit=2&offset=2", admin).json(t)
	eq(t, versions(t, page2), "2.0.0,0.1.0")
	eq(t, page2["total"], any(float64(4)))

	envs := app.get(recordsPath(pid, "environments"), admin).json(t)
	var rows []string
	for _, e := range list(t, envs, "items") {
		m := e.(obj)
		rows = append(rows, fmt.Sprintf("%s/%s=%s@%s", m["service"], m["environment"], m["version"], m["branch"]))
	}
	eq(t, strings.Join(rows, ","), "api/production=2.1.0@main,api/staging=2.1.0@feature/x,worker/production=0.1.0@main")
	eq(t, at(envs, "items", 0, "occurred_at"), any("2025-06-01T12:00:00Z"))
}

func TestEnvironmentsWithClusterObservations(t *testing.T) {
	t.Parallel()
	app, k8s := startClusterApp(t)
	root := app.admin()
	project, key := app.projectWithKey(root, "api")
	app.cluster(root, "prod", nil)
	for i, env := range []string{"perf", "development", "production"} {
		eq(t, app.ingest(project, key, deployed("k"+env, "api", env, "1.4.2", "2026-10-01T1"+string(rune('0'+i))+":00:00Z")).status, 201)
	}
	ann := func(env string) map[string]string {
		return map[string]string{deploy.AnnotationProject: "api-org/api", deploy.AnnotationService: "api", deploy.AnnotationEnvironment: env}
	}
	k8s.Cluster().Put(workload("u1", "api", "1.4.1", ann("production")), workload("u2", "api-stg", "2.0.0", ann("staging")))
	app.pollNow()

	r := app.get(nodePath(project)+"/environments", root)
	eq(t, r.status, 200, r.text())
	items := list(t, r.json(t), "items")
	var envs []string
	for _, it := range items {
		envs = append(envs, it.(obj)["environment"].(string))
	}
	eq(t, strings.Join(envs, ","), "production,staging,development,perf", "directory order, then the rest")
	prod := items[0].(obj)
	eq(t, prod["version"], any("1.4.2"))
	eq(t, prod["source"], any("event"))
	eq(t, prod["drift"], any(true))
	observed := list(t, prod, "observed")[0].(obj)
	eq(t, observed["version"], any("1.4.1"))
	eq(t, observed["cluster"], any("prod"))
	eq(t, observed["workload"], any("api"))
	eq(t, at(observed, "replicas", "ready"), any(float64(2)))
	eq(t, observed["rollout"], any("complete"))
	staging := items[1].(obj)
	eq(t, staging["version"], nil)
	eq(t, staging["deployment_id"], nil)
	eq(t, staging["drift"], any(false))
	eq(t, len(list(t, staging, "observed")), 1)
	eq(t, len(list(t, items[2].(obj), "observed")), 0)

	eq(t, list(t, app.get(nodePath(project)+"/deployments", root).json(t), "items")[0].(obj)["source"], any("event"))

	eq(t, app.send("PATCH", nodePath(project), root, obj{"cluster_observation": false}).status, 200)
	items = list(t, app.get(nodePath(project)+"/environments", root).json(t), "items")
	eq(t, len(items), 3)
	eq(t, items[0].(obj)["drift"], any(false))
}
