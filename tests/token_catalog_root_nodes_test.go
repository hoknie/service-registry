package tests

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func tokenNodeIDs(t *testing.T, app *testApp, cookie string) []string {
	t.Helper()
	ids := []string{}
	for _, n := range app.get(tokensPath, cookie).json(t)["items"].([]any)[0].(obj)["nodes"].([]any) {
		ids = append(ids, n.(obj)["id"].(string))
	}
	return ids
}

func TestCatalogRootOfANodeTokenAddsTheOrganizationsItCreates(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	app.user("chief@example.com", true)
	chief := app.session("chief@example.com", password)
	secret := app.secretOn(chief, []string{tr.acme}, "read", "write", "catalog.root")
	org := func(slug string) obj {
		return obj{"kind": "organization", "parent_id": nil, "slug": slug, "name": slug}
	}

	r := app.bearer("POST", "/api/v1/catalog/nodes", secret, org("beta"))
	eq(t, r.status, 201, r.text())
	beta := idOf(r.json(t))
	eq(t, strings.Join(tokenNodeIDs(t, app, chief), ","), tr.acme+","+beta)
	eq(t, app.bearer("GET", nodePath(beta), secret, nil).status, 200)

	r = app.bearer("POST", nodePath(tr.backend)+"/move", secret, obj{"parent_id": nil})
	eq(t, code(t, r), any("auth.insufficient_scope"), "only roots of the token move at the top level")
	r = app.bearer("DELETE", nodePath(tr.globex), secret, nil)
	eq(t, r.status, 404, "roots outside the token stay invisible")
	eq(t, app.bearer("DELETE", nodePath(beta), secret, nil).status, 204)

	plain := app.secretOn(chief, []string{tr.acme}, "read", "write")
	r = app.bearer("POST", "/api/v1/catalog/nodes", plain, org("gamma"))
	eq(t, code(t, r), any("auth.insufficient_scope"))
}

func TestCatalogRootOfANodeTokenStopsAtTheNodeLimit(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	secret := app.secretOn(tr.admin, []string{tr.acme}, "write", "catalog.root")
	for i := range 19 {
		slug := fmt.Sprintf("org-%d", i)
		r := app.bearer("POST", "/api/v1/catalog/nodes", secret, obj{"kind": "organization", "parent_id": nil, "slug": slug, "name": slug})
		eq(t, r.status, 201, r.text())
	}
	r := app.bearer("POST", "/api/v1/catalog/nodes", secret, obj{"kind": "organization", "parent_id": nil, "slug": "full", "name": "full"})
	eq(t, r.status, 409, r.text())
	eq(t, code(t, r), any("conflict.token_nodes_full"))
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM nodes WHERE slug = 'full'"), int64(0), "the organization is not kept")
	eq(t, len(tokenNodeIDs(t, app, tr.admin)), 20)
}

func TestCatalogRootOfANodeTokenNeedsACurrentSuperadmin(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	chief := app.user("chief@example.com", true)
	chiefCookie := app.session("chief@example.com", password)
	secret := app.secretOn(chiefCookie, []string{tr.acme}, "write", "catalog.root")
	eq(t, app.send("PATCH", "/api/v1/users/"+chief.ID.String(), tr.admin, obj{"is_superadmin": false}).status, 200)
	r := app.bearer("POST", "/api/v1/catalog/nodes", secret, obj{"kind": "organization", "parent_id": nil, "slug": "beta", "name": "Beta"})
	eq(t, code(t, r), any("auth.forbidden"), r.text())
	eq(t, slices.Contains(tokenNodeIDs(t, app, chiefCookie), tr.acme), true)
}
