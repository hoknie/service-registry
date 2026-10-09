package tests

import (
	"net/url"
	"strings"
	"testing"
)

func tablePath(query string) string { return "/api/v1/catalog/table?" + query }

func names(t *testing.T, r reply) string {
	t.Helper()
	var out []string
	for _, item := range r.json(t)["items"].([]any) {
		m := item.(obj)
		name := m["name"].(string)
		if m["match"] == false {
			name = "(" + name + ")"
		}
		out = append(out, name)
	}
	return strings.Join(out, ",")
}

type tableTree struct {
	*testApp
	admin, acme, backend, web, billing, users string
}

func startTable(t *testing.T) *tableTree {
	t.Helper()
	a := startForgeApp(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	backend := a.nodeID(admin, "folder", acme, "backend")
	web := a.nodeID(admin, "folder", acme, "web")
	r := a.createNode(admin, obj{"kind": "project", "parent_id": backend, "slug": "billing-api", "name": "Billing API",
		"labels": obj{"team": "payments", "tier": "gold"}})
	eq(t, r.status, 201, r.text())
	users := a.nodeID(admin, "project", backend, "users-api")
	a.nodeID(admin, "project", web, "site")
	a.nodeID(admin, "organization", "", "zeta")
	return &tableTree{testApp: a, admin: admin, acme: acme, backend: backend, web: web, billing: idOf(r.json(t)), users: users}
}

func TestCatalogTableChildrenPageWithCounts(t *testing.T) {
	t.Parallel()
	a := startTable(t)
	r := a.get(tablePath(""), a.admin)
	eq(t, r.status, 200, r.text())
	eq(t, names(t, r), "acme,zeta")
	eq(t, at(r.json(t), "items", 0, "children"), any(float64(2)))
	eq(t, jsonText(at(r.json(t), "items", 0, "activity")), `{"failed":0,"queued":0,"running":0}`)
	eq(t, r.json(t)["truncated"], any(false))

	r = a.get(tablePath("parent="+a.backend), a.admin)
	eq(t, names(t, r), "Billing API,users-api")
	eq(t, at(r.json(t), "items", 0, "children"), any(float64(0)))
	eq(t, at(r.json(t), "items", 0, "match"), any(true))
	eq(t, jsonText(at(r.json(t), "items", 0, "activity")), `[]`)
	eq(t, at(r.json(t), "items", 0, "parent_id"), any(a.backend))

	r = a.get(tablePath("parent="+a.backend+"&limit=1&offset=1"), a.admin)
	eq(t, names(t, r), "users-api")
	eq(t, r.json(t)["total"], any(float64(2)))
	eq(t, a.get(tablePath("parent=nope"), a.admin).status, 404)
	eq(t, a.get(tablePath("limit=0"), a.admin).json(t)["code"], any("validation.invalid_pagination"))
}

func TestCatalogTableSearchKeepsAncestors(t *testing.T) {
	t.Parallel()
	a := startTable(t)
	r := a.get(tablePath("q=BILL"), a.admin)
	eq(t, r.status, 200, r.text())
	eq(t, names(t, r), "(acme),(backend),Billing API")
	eq(t, at(r.json(t), "items", 1, "depth"), any(float64(2)))

	eq(t, names(t, a.get(tablePath("q=api"), a.admin)), "(acme),(backend),Billing API,users-api")
	eq(t, names(t, a.get(tablePath("kind=folder"), a.admin)), "(acme),backend,web")
	eq(t, names(t, a.get(tablePath("label=tier"), a.admin)), "(acme),(backend),Billing API")
	eq(t, names(t, a.get(tablePath("label=team%3Dpayments&label=tier%3Dgold"), a.admin)), "(acme),(backend),Billing API")
	eq(t, names(t, a.get(tablePath("label=team%3Dother"), a.admin)), "")
	eq(t, names(t, a.get(tablePath("q=site&parent="+a.acme), a.admin)), "(web),site")
	eq(t, names(t, a.get(tablePath("q="+url.QueryEscape("z")), a.admin)), "zeta")

	for _, bad := range []string{"kind=service", "activity=idle", "label=Team", "q=" + strings.Repeat("a", 101),
		"label=a&label=b&label=c&label=d&label=e&label=f"} {
		r := a.get(tablePath(bad), a.admin)
		eq(t, r.status, 400, bad)
		eq(t, r.json(t)["code"], any("validation.invalid_filter"), bad)
	}
}

func TestCatalogTableActivityFilter(t *testing.T) {
	t.Parallel()
	a := startTable(t)
	s := &sourcesApp{testApp: a.testApp, admin: a.admin, org: a.acme}
	s.failForge(a.users, "forge.unauthorized")
	r := a.get(tablePath("activity=failed"), a.admin)
	eq(t, r.status, 200, r.text())
	eq(t, names(t, r), "(acme),(backend),users-api")
	eq(t, at(r.json(t), "items", 2, "activity", 1, "kind"), any("forge"))
	eq(t, at(r.json(t), "items", 2, "activity", 1, "state"), any("failed"))
	eq(t, at(r.json(t), "items", 2, "activity", 1, "pending"), nil)
	eq(t, jsonText(at(r.json(t), "items", 1, "activity")), `{"failed":1,"queued":0,"running":0}`)
	eq(t, names(t, a.get(tablePath("activity=running"), a.admin)), "")
}

func TestCatalogTableVisibilityAndTruncation(t *testing.T) {
	t.Parallel()
	a := startTable(t)
	a.user("viewer@example.com", false)
	viewer := a.session("viewer@example.com", password)
	a.grant(a.admin, a.backend, "user", "viewer@example.com", "viewer")
	r := a.get(tablePath(""), viewer)
	eq(t, names(t, r), "acme")
	eq(t, at(r.json(t), "items", 0, "access"), any("navigate"))
	eq(t, at(r.json(t), "items", 0, "children"), any(float64(1)))
	eq(t, at(r.json(t), "items", 0, "labels"), nil)
	eq(t, names(t, a.get(tablePath("parent="+a.acme), viewer)), "backend")
	eq(t, names(t, a.get(tablePath("q=s"), viewer)), "(acme),(backend),users-api")
	eq(t, names(t, a.get(tablePath("q=acme"), viewer)), "acme")
	eq(t, a.get(tablePath("parent="+a.web), viewer).status, 404)

	execSQL(t, a.db, `INSERT INTO nodes (id, kind, parent_id, slug, name)
		SELECT gen_random_uuid(), 'project', $1, 'many-' || g, 'Many ' || lpad(g::text, 3, '0')
		FROM generate_series(1, 501) g`, a.web)
	r = a.get(tablePath("q=many"), a.admin)
	eq(t, r.json(t)["truncated"], any(true))
	items := r.json(t)["items"].([]any)
	eq(t, len(items), 502)
	eq(t, at(items[501], "name"), any("Many 500"))
}

func TestCatalogTableRowsCarryLinkIcons(t *testing.T) {
	t.Parallel()
	a := startTable(t)
	a.putTemplate(a.admin, a.acme, "grafana", "https://grafana.example/d/{project.slug}")
	a.putTemplate(a.admin, a.acme, "logs", "https://logs.example/?ns={namespace}")
	before := scalar[int64](t, a.db, "SELECT count(*) FROM link_targets")
	r := a.get(tablePath("parent="+a.backend), a.admin)
	eq(t, r.status, 200, r.text())
	row := at(r.json(t), "items", 0).(obj)
	eq(t, row["name"], any("Billing API"))
	items := row["links"].([]any)
	eq(t, len(items), 2)
	byKey := map[string]obj{}
	for _, it := range items {
		byKey[it.(obj)["link_key"].(string)] = it.(obj)
	}
	eq(t, byKey["grafana"]["url"], any("https://grafana.example/d/billing-api"))
	eq(t, at(byKey["grafana"], "icon", "kind"), any("builtin"))
	eq(t, byKey["logs"]["url"], nil)
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM link_targets"), before, "icons do not register addresses")
	eq(t, jsonText(at(a.get(tablePath(""), a.admin).json(t), "items", 0, "links")), `[]`, "containers have no links")
}
