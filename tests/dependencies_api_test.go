package tests

import (
	"strings"
	"testing"
)

type depsTree struct {
	*testApp
	admin, acme, backend, other     string
	api, billing, auth, secret, web string
	apiKey                          string
}

func depsPath(id string) string { return nodePath(id) + "/dependencies" }

func startDeps(t *testing.T) *depsTree {
	t.Helper()
	a := startApp(t)
	d := &depsTree{testApp: a, admin: a.admin()}
	d.acme = d.nodeID(d.admin, "organization", "", "acme")
	d.backend = d.nodeID(d.admin, "folder", d.acme, "backend")
	d.other = d.nodeID(d.admin, "folder", d.acme, "other")
	api := d.node(d.admin, "project", d.backend, "api")
	d.api, d.apiKey = idOf(api), at(api, "key", "secret").(string)
	d.billing = d.nodeID(d.admin, "project", d.backend, "billing")
	d.auth = d.nodeID(d.admin, "project", d.other, "auth")
	hidden := d.nodeID(d.admin, "organization", "", "hidden")
	d.secret = d.nodeID(d.admin, "project", hidden, "secret")
	d.web = d.nodeID(d.admin, "project", d.acme, "web")
	return d
}

func (d *depsTree) deploy(key, env, at string, deps any) reply {
	d.t.Helper()
	body := deployed(key, "api", env, "1."+key, at)
	if deps != nil {
		body["payload"].(obj)["dependencies"] = deps
	}
	return d.ingest(d.api, d.apiKey, body)
}

func targets(t *testing.T, r reply) string {
	t.Helper()
	var out []string
	for _, it := range list(t, r.json(t), "depends_on") {
		target := it.(obj)["target"].(obj)
		var srcs []string
		for _, s := range it.(obj)["sources"].([]any) {
			srcs = append(srcs, s.(obj)["source"].(string))
		}
		label := target["type"].(string)
		for _, k := range []string{"path", "name", "ref"} {
			if v, ok := target[k]; ok {
				label += ":" + v.(string)
				break
			}
		}
		out = append(out, label+"["+strings.Join(srcs, "+")+"]")
	}
	return strings.Join(out, ",")
}

func TestDependenciesFromEvents(t *testing.T) {
	t.Parallel()
	d := startDeps(t)
	r := d.deploy("bad", "production", "2026-10-01T09:00:00Z", []any{"acme/backend/billing", 7, "external:"})
	eq(t, r.status, 422, r.text())
	eq(t, jsonText(r.json(t)["fields"]), `[{"code":"invalid_type","path":"payload.dependencies.1"},{"code":"invalid_value","path":"payload.dependencies.2"}]`)
	eq(t, d.deploy("bad2", "production", "2026-10-01T09:00:00Z", "x").status, 422)

	r = d.deploy("a", "production", "2026-10-01T10:00:00Z", []string{"ACME/Backend/Billing", "external: Redis", "acme/nope", "acme/backend/api"})
	eq(t, r.status, 201, r.text())
	eq(t, targets(t, d.get(depsPath(d.api), d.admin)), "project:acme/backend/billing[event],external:redis[event],unresolved:acme/nope[event]")
	eq(t, scalar[string](t, d.db, "SELECT origin FROM service_dependencies LIMIT 1"), "api/production")

	eq(t, d.deploy("b", "production", "2026-10-01T09:30:00Z", []string{}).status, 201)
	eq(t, targets(t, d.get(depsPath(d.api), d.admin)), "project:acme/backend/billing[event],external:redis[event],unresolved:acme/nope[event]", "late event")
	eq(t, d.deploy("c", "production", "2026-10-01T11:00:00Z", nil).status, 201)
	eq(t, d.count("service_dependencies"), int64(3), "field not given")
	eq(t, d.deploy("s", "staging", "2026-10-01T11:00:00Z", []string{}).status, 201)
	eq(t, d.count("service_dependencies"), int64(3), "other environment")

	eq(t, d.deploy("d", "production", "2026-10-01T12:00:00Z", []string{"acme/backend/billing"}).status, 201)
	eq(t, targets(t, d.get(depsPath(d.api), d.admin)), "project:acme/backend/billing[event]")
	eq(t, d.deploy("d", "production", "2026-10-01T12:00:00Z", []string{"acme/backend/billing"}).status, 200, "replay")

	r = d.get(depsPath(d.billing), d.admin)
	eq(t, at(r.json(t), "dependents", 0, "project", "path"), any("acme/backend/api"))
	eq(t, d.get(depsPath(d.backend), d.admin).status, 404)
}

func TestManualDependenciesAndVisibility(t *testing.T) {
	t.Parallel()
	d := startDeps(t)
	d.user("ed@example.com", false)
	d.user("vi@example.com", false)
	d.grant(d.admin, d.backend, "user", "ed@example.com", "editor")
	d.grant(d.admin, d.backend, "user", "vi@example.com", "viewer")
	ed, vi := d.session("ed@example.com", password), d.session("vi@example.com", password)

	r := d.send("POST", depsPath(d.api), ed, obj{"target": "acme/backend/billing", "note": "оплата"})
	eq(t, r.status, 201, r.text())
	eq(t, at(r.json(t), "sources", 0, "source"), any("manual"))
	eq(t, at(r.json(t), "sources", 0, "note"), any("оплата"))
	manual := at(r.json(t), "sources", 0, "id").(string)
	eq(t, code(t, d.send("POST", depsPath(d.api), ed, obj{"target": "acme/backend/billing"})), any("conflict.dependency_exists"))
	for _, bad := range []string{"acme/other/auth", "acme/nope", "hidden/secret", "external:", "acme/backend/api"} {
		eq(t, code(t, d.send("POST", depsPath(d.api), ed, obj{"target": bad})), any("validation.invalid_dependency"), bad)
	}
	eq(t, d.send("POST", depsPath(d.api), ed, obj{"target": "external:stripe"}).status, 201)
	eq(t, code(t, d.send("POST", depsPath(d.api), vi, obj{"target": "external:x"})), any("auth.forbidden"))

	eq(t, d.deploy("a", "production", "2026-10-01T10:00:00Z", []string{"acme/backend/billing", "hidden/secret", "acme/other/auth"}).status, 201)
	eq(t, d.send("POST", depsPath(d.web), d.admin, obj{"target": "acme/backend/billing"}).status, 201)
	execSQL(t, d.db, "INSERT INTO service_dependencies (id, project_id, source, origin, target_project_id, target_ref) VALUES (gen_random_uuid(), $1, 'manual', '', $2, 'acme/backend/billing')", d.secret, d.billing)

	eq(t, targets(t, d.get(depsPath(d.api), vi)), "project:acme/backend/billing[manual+event],external:stripe[manual],hidden[event],hidden[event]")
	r = d.get(depsPath(d.billing), vi)
	eq(t, len(list(t, r.json(t), "dependents")), 1)
	eq(t, r.json(t)["hidden_dependents"], any(float64(2)))

	auto := scalar[string](t, d.db, "SELECT id::text FROM service_dependencies WHERE source = 'event' LIMIT 1")
	eq(t, d.send("DELETE", depsPath(d.api)+"/"+auto, ed, nil).status, 404)
	eq(t, code(t, d.send("DELETE", depsPath(d.api)+"/"+manual, vi, nil)), any("auth.forbidden"))
	eq(t, d.send("DELETE", depsPath(d.api)+"/"+manual, ed, nil).status, 204)
	eq(t, targets(t, d.get(depsPath(d.api), ed)), "project:acme/backend/billing[event],external:stripe[manual],hidden[event],hidden[event]")
	eq(t, d.get(depsPath(d.secret), vi).status, 404)
}

func TestDependencyGraphs(t *testing.T) {
	t.Parallel()
	d := startDeps(t)
	d.user("vi@example.com", false)
	d.grant(d.admin, d.acme, "user", "vi@example.com", "viewer")
	vi := d.session("vi@example.com", password)
	eq(t, d.deploy("a", "production", "2026-10-01T10:00:00Z", []string{"acme/backend/billing", "acme/other/auth"}).status, 201)
	eq(t, d.send("POST", depsPath(d.billing), d.admin, obj{"target": "external:postgres"}).status, 201)
	eq(t, d.send("POST", depsPath(d.secret), d.admin, obj{"target": "acme/backend/api"}).status, 201)

	g := d.get(depsPath(d.api)+"/graph?depth=2", vi).json(t)
	var ids []string
	for _, n := range list(t, g, "nodes") {
		ids = append(ids, n.(obj)["name"].(string))
	}
	eq(t, strings.Join(ids, ","), "api,billing,auth,postgres")
	eq(t, g["hidden_edges"], any(float64(1)))
	eq(t, len(list(t, g, "edges")), 3)
	eq(t, at(g, "nodes", 0, "in_scope"), any(true))
	eq(t, at(g, "nodes", 1, "in_scope"), any(false))

	g = d.get(depsPath(d.api)+"/graph", vi).json(t)
	eq(t, len(list(t, g, "nodes")), 3)

	g = d.get(depsPath(d.backend)+"/graph", vi).json(t)
	scope := map[string]bool{}
	for _, n := range list(t, g, "nodes") {
		scope[n.(obj)["name"].(string)] = n.(obj)["in_scope"].(bool)
	}
	eq(t, jsonText(scope), `{"api":true,"auth":false,"billing":true,"postgres":false}`)
	eq(t, code(t, d.get(depsPath(d.api)+"/graph?depth=5", vi)), any("validation.invalid_dependency_depth"))
	eq(t, code(t, d.get(depsPath(d.api)+"/graph?depth=x", vi)), any("validation.invalid_dependency_depth"))
}

func TestDependenciesSurviveTargetMovesAndDeletes(t *testing.T) {
	t.Parallel()
	d := startDeps(t)
	eq(t, d.deploy("a", "production", "2026-10-01T10:00:00Z", []string{"acme/backend/billing"}).status, 201)
	eq(t, d.send("POST", depsPath(d.web), d.admin, obj{"target": "acme/backend/api"}).status, 201)
	eq(t, d.send("POST", nodePath(d.billing)+"/move", d.admin, obj{"parent_id": d.other}).status, 200)
	eq(t, targets(t, d.get(depsPath(d.api), d.admin)), "project:acme/other/billing[event]")
	eq(t, d.send("DELETE", nodePath(d.billing), d.admin, nil).status, 204)
	eq(t, targets(t, d.get(depsPath(d.api), d.admin)), "unresolved:acme/other/billing[event]")
	eq(t, d.send("DELETE", nodePath(d.web), d.admin, nil).status, 204)
	eq(t, len(list(t, d.get(depsPath(d.api), d.admin).json(t), "dependents")), 0)
	_, err := d.db.Pool.Exec(t.Context(), "INSERT INTO service_dependencies (id, project_id, source, origin, target_project_id, target_ref) VALUES (gen_random_uuid(), $1, 'manual', '', $1, 'x')", d.api)
	if err == nil || !strings.Contains(err.Error(), "service_dependencies_self_check") {
		t.Fatal(err)
	}
}

func TestDependencyTargetPathsFollowAncestors(t *testing.T) {
	t.Parallel()
	d := startDeps(t)
	eq(t, d.deploy("a", "production", "2026-10-01T10:00:00Z", []string{"acme/backend/billing"}).status, 201)
	eq(t, d.send("POST", depsPath(d.web), d.admin, obj{"target": "acme/backend/billing"}).status, 201)
	eq(t, d.send("PATCH", nodePath(d.backend), d.admin, obj{"slug": "core"}).status, 200)
	eq(t, d.send("PATCH", nodePath(d.acme), d.admin, obj{"slug": "corp"}).status, 200)
	eq(t, scalar[int64](t, d.db, "SELECT count(*) FROM service_dependencies WHERE target_ref = 'corp/core/billing'"), int64(2))
	eq(t, d.send("DELETE", nodePath(d.billing), d.admin, nil).status, 204)
	eq(t, targets(t, d.get(depsPath(d.api), d.admin)), "unresolved:corp/core/billing[event]")
	eq(t, targets(t, d.get(depsPath(d.web), d.admin)), "unresolved:corp/core/billing[manual]")
}

func TestDeletingTargetMergesMatchingUnresolvedEdge(t *testing.T) {
	t.Parallel()
	d := startDeps(t)
	eq(t, d.send("POST", depsPath(d.api), d.admin, obj{"target": "acme/backend/billing", "note": "оплата"}).status, 201)
	eq(t, d.send("DELETE", nodePath(d.billing), d.admin, nil).status, 204)
	eq(t, targets(t, d.get(depsPath(d.api), d.admin)), "unresolved:acme/backend/billing[manual]")
	d.billing = d.nodeID(d.admin, "project", d.backend, "billing")
	eq(t, d.send("POST", depsPath(d.api), d.admin, obj{"target": "acme/backend/billing"}).status, 201)
	eq(t, targets(t, d.get(depsPath(d.api), d.admin)), "project:acme/backend/billing[manual],unresolved:acme/backend/billing[manual]")
	kept := scalar[string](t, d.db, "SELECT id::text FROM service_dependencies WHERE target_project_id IS NOT NULL")
	eq(t, d.send("DELETE", nodePath(d.billing), d.admin, nil).status, 204)
	r := d.get(depsPath(d.api), d.admin)
	eq(t, targets(t, r), "unresolved:acme/backend/billing[manual]")
	eq(t, at(r.json(t), "depends_on", 0, "sources", 0, "id"), any(kept))
	eq(t, at(r.json(t), "depends_on", 0, "sources", 0, "note"), any("оплата"))
}
