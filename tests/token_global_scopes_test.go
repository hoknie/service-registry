package tests

import (
	"slices"
	"testing"
)

func TestGlobalScopesOpenOnlyTheirRoutes(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	routes := []struct {
		scope, method, path string
		status              int
	}{
		{"users:read", "GET", "/api/v1/users", 200},
		{"users:write", "POST", "/api/v1/users", 400},
		{"groups:read", "GET", "/api/v1/groups", 200},
		{"groups:write", "POST", "/api/v1/groups", 400},
		{"clusters:read", "GET", "/api/v1/clusters", 200},
		{"clusters:write", "POST", "/api/v1/clusters", 400},
		{"settings:read", "GET", "/api/v1/secrets", 200},
		{"settings:read", "GET", "/api/v1/knowledge/scans", 200},
		{"settings:write", "POST", "/api/v1/secrets", 400},
		{"settings:write", "POST", "/api/v1/link-kinds", 400},
		{"settings:write", "POST", "/api/v1/environments", 400},
	}
	scopes := []string{"users:read", "users:write", "groups:read", "groups:write", "clusters:read", "clusters:write",
		"settings:read", "settings:write", "catalog.root"}
	secrets := map[string]string{}
	for _, s := range scopes {
		secrets[s] = app.pat(root, s)
	}
	for _, r := range routes {
		for _, s := range scopes {
			var body any
			if r.method == "POST" {
				body = obj{}
			}
			got := app.bearer(r.method, r.path, secrets[s], body)
			if s == r.scope {
				eq(t, got.status, r.status, s+" "+r.method+" "+r.path+" "+got.text())
				continue
			}
			eq(t, got.status, 403, s+" "+r.method+" "+r.path)
			eq(t, code(t, got), any("auth.insufficient_scope"), s+" "+r.path)
		}
	}
	for _, path := range []string{"/api/v1/link-kinds", "/api/v1/environments"} {
		eq(t, code(t, app.bearer("POST", path, app.pat(root, "read", "write"), obj{})), any("auth.insufficient_scope"), path)
	}
}

func TestGlobalScopesNeedACurrentSuperadmin(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	root := tr.admin
	bob := bobSession(app)
	for _, s := range []string{"users:read", "users:write", "groups:read", "groups:write", "clusters:read", "clusters:write",
		"settings:read", "settings:write", "catalog.root"} {
		r := app.issue(bob, obj{"name": "t", "scopes": []string{"read", s}, "expires_in_days": 30})
		eq(t, code(t, r), any("validation.invalid_scopes"), s)
		if s == "catalog.root" {
			continue
		}
		r = app.issue(root, obj{"name": "t", "scopes": []string{"read", s}, "expires_in_days": 30, "nodes": []string{tr.acme}})
		eq(t, code(t, r), any("validation.invalid_token_nodes"), s)
	}
	for _, s := range []string{"admin", "users", "groups", "clusters", "settings"} {
		eq(t, code(t, app.issue(root, obj{"name": "t", "scopes": []string{s}, "expires_in_days": 30})), any("validation.invalid_scopes"), s)
	}

	chief := app.user("chief@example.com", true)
	chiefCookie := app.session("chief@example.com", password)
	people := app.pat(chiefCookie, "users:read", "write", "catalog.root")
	issued := app.get(tokensPath, chiefCookie).json(t)["items"].([]any)[0].(obj)
	eq(t, slices.Equal(stringsOf(issued["scopes"]), []string{"write", "users:read", "catalog.root"}), true, issued["scopes"])
	eq(t, issued["catalog_admin"], any(true))
	eq(t, app.bearer("GET", "/api/v1/users", people, nil).status, 200)
	eq(t, app.send("PATCH", "/api/v1/users/"+chief.ID.String(), root, obj{"is_superadmin": false}).status, 200)
	r := app.bearer("GET", "/api/v1/users", people, nil)
	eq(t, code(t, r), any("auth.forbidden"), r.text())
	r = app.bearer("POST", "/api/v1/catalog/nodes", people, obj{"kind": "organization", "parent_id": nil, "slug": "beta", "name": "Beta"})
	eq(t, code(t, r), any("auth.forbidden"))
	eq(t, app.get("/api/v1/users/"+chief.ID.String()+"/tokens", root).json(t)["items"].([]any)[0].(obj)["catalog_admin"], any(false),
		"the catalog role label follows the current flag")
}

func stringsOf(v any) []string {
	out := []string{}
	for _, s := range v.([]any) {
		out = append(out, s.(string))
	}
	return out
}
