package tests

import (
	"testing"

	"github.com/google/uuid"

	"svc-registry/pkg/testing/oidcfake"
)

type rightsFixture struct {
	app   *testApp
	admin string
	group string
	bob   string
	bobID string
}

func newRightsFixture(t *testing.T) rightsFixture {
	t.Helper()
	app := startApp(t)
	admin := app.admin()
	bob := app.user("bob@example.com", false)
	group := idOf(createGroup(t, app, admin, "Ops"))
	eq(t, app.call("PUT", "/api/v1/groups/"+group+"/members/"+bob.ID.String(), admin).status, 204)
	return rightsFixture{app: app, admin: admin, group: group, bob: app.session("bob@example.com", password), bobID: bob.ID.String()}
}

func (f rightsFixture) grant(t *testing.T, rights obj) reply {
	t.Helper()
	r := f.app.send("PUT", "/api/v1/groups/"+f.group+"/permissions", f.admin, obj{"permissions": rights})
	eq(t, r.status, 200, r.text())
	return r
}

func TestGroupPermissionsAreValidatedAndSetBySuperadminOnly(t *testing.T) {
	t.Parallel()
	f := newRightsFixture(t)
	r := f.grant(t, obj{"clusters": []string{"update", "read"}, "scans": []string{"read"}, "users": []string{}})
	eq(t, jsonText(r.json(t)), `{"permissions":{"clusters":["read","update"],"scans":["read"]}}`)
	eq(t, jsonText(f.app.get("/api/v1/groups/"+f.group, f.admin).json(t)["permissions"]), `{"clusters":["read","update"],"scans":["read"]}`)
	for _, bad := range []obj{
		{"scans": []string{"delete"}},
		{"billing": []string{"read"}},
		{"clusters": []string{"read", "read"}},
		{"environments": []string{"read"}},
	} {
		r := f.app.send("PUT", "/api/v1/groups/"+f.group+"/permissions", f.admin, obj{"permissions": bad})
		eq(t, r.status, 400, jsonText(bad))
		eq(t, code(t, r), any("validation.invalid_permissions"))
	}
	eq(t, f.app.send("PUT", "/api/v1/groups/"+f.group+"/permissions", f.admin, obj{}).status, 400)
	eq(t, f.app.send("PUT", "/api/v1/groups/"+uuid.Must(uuid.NewV7()).String()+"/permissions", f.admin, obj{"permissions": obj{}}).status, 404)

	f.grant(t, obj{"groups": []string{"read", "update"}})
	r = f.app.send("PUT", "/api/v1/groups/"+f.group+"/permissions", f.bob, obj{"permissions": obj{"users": []string{"read"}}})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	f.grant(t, obj{})
	eq(t, jsonText(f.app.get("/api/v1/groups/"+f.group, f.admin).json(t)["permissions"]), `{}`)
}

func TestEachSectionRightOpensItsRoutes(t *testing.T) {
	t.Parallel()
	f := newRightsFixture(t)
	carol := f.app.user("carol@example.com", false)
	plain := f.app.session("carol@example.com", password)
	id := uuid.Must(uuid.NewV7()).String()
	for _, rt := range []struct {
		section, action, method, path string
		body                          any
	}{
		{"users", "read", "GET", "/api/v1/users", nil},
		{"users", "read", "GET", "/api/v1/users/" + carol.ID.String(), nil},
		{"users", "read", "GET", "/api/v1/users/" + carol.ID.String() + "/identities", nil},
		{"users", "read", "GET", "/api/v1/users/" + carol.ID.String() + "/groups", nil},
		{"users", "create", "POST", "/api/v1/users", obj{"email": "new@example.com", "display_name": "New", "password": "long-enough-1"}},
		{"users", "update", "PATCH", "/api/v1/users/" + id, obj{"display_name": "X"}},
		{"users", "update", "POST", "/api/v1/users/" + id + "/password", obj{"password": "long-enough-1"}},
		{"groups", "read", "GET", "/api/v1/groups", nil},
		{"groups", "read", "GET", "/api/v1/groups/" + id, nil},
		{"groups", "create", "POST", "/api/v1/groups", obj{"name": "Fresh"}},
		{"groups", "update", "PATCH", "/api/v1/groups/" + id, obj{"name": "X"}},
		{"groups", "update", "PUT", "/api/v1/groups/" + id + "/members/" + carol.ID.String(), nil},
		{"groups", "update", "DELETE", "/api/v1/groups/" + id + "/members/" + carol.ID.String(), nil},
		{"groups", "update", "PUT", "/api/v1/groups/" + id + "/contacts", obj{"contacts": []any{}, "on_call_url": nil}},
		{"groups", "update", "PUT", "/api/v1/groups/" + id + "/contact-editors/" + carol.ID.String(), nil},
		{"groups", "delete", "DELETE", "/api/v1/groups/" + id, nil},
		{"tokens", "read", "GET", "/api/v1/tokens", nil},
		{"tokens", "delete", "DELETE", "/api/v1/tokens/" + id, nil},
		{"clusters", "read", "GET", "/api/v1/clusters", nil},
		{"clusters", "read", "GET", "/api/v1/clusters/" + id, nil},
		{"clusters", "read", "GET", "/api/v1/clusters/" + id + "/unmatched", nil},
		{"clusters", "create", "POST", "/api/v1/clusters", obj{}},
		{"clusters", "update", "PATCH", "/api/v1/clusters/" + id, obj{}},
		{"clusters", "update", "POST", "/api/v1/clusters/" + id + "/test", nil},
		{"clusters", "update", "POST", "/api/v1/clusters/" + id + "/poll", nil},
		{"clusters", "update", "POST", "/api/v1/clusters/" + id + "/agent-token", obj{}},
		{"clusters", "update", "DELETE", "/api/v1/clusters/" + id + "/agent-token", nil},
		{"clusters", "delete", "DELETE", "/api/v1/clusters/" + id, nil},
		{"environments", "create", "POST", "/api/v1/environments", obj{"key": "qa", "names": obj{"en": "QA"}}},
		{"environments", "update", "PATCH", "/api/v1/environments/nope", obj{}},
		{"environments", "delete", "DELETE", "/api/v1/environments/nope", nil},
		{"link_kinds", "create", "POST", "/api/v1/link-kinds", obj{"key": "wiki", "names": obj{"en": "Wiki"}}},
		{"link_kinds", "update", "PATCH", "/api/v1/link-kinds/nope", obj{}},
		{"link_kinds", "update", "PUT", "/api/v1/link-kinds", obj{"keys": []string{}}},
		{"link_kinds", "delete", "DELETE", "/api/v1/link-kinds/nope", nil},
		{"secrets", "read", "GET", "/api/v1/secrets", nil},
		{"secrets", "read", "GET", "/api/v1/secrets/" + id, nil},
		{"secrets", "create", "POST", "/api/v1/secrets", obj{}},
		{"secrets", "update", "PATCH", "/api/v1/secrets/" + id, obj{}},
		{"secrets", "delete", "DELETE", "/api/v1/secrets/" + id, nil},
		{"scans", "read", "GET", "/api/v1/knowledge/scans", nil},
	} {
		name := rt.method + " " + rt.path
		r := f.app.send(rt.method, rt.path, plain, rt.body)
		eq(t, denied(t, r), true, "no rights: ", name, " ", r.text())
		others := obj{}
		for _, other := range []string{"create", "read", "update", "delete"} {
			if other != rt.action && applies(rt.section, other) {
				others[rt.section] = append(asStrings(others[rt.section]), other)
			}
		}
		if len(others) > 0 {
			f.grant(t, others)
			r := f.app.send(rt.method, rt.path, f.bob, rt.body)
			eq(t, denied(t, r), true, "other actions only: ", name, " ", r.text())
		}
		f.grant(t, obj{rt.section: []string{rt.action}})
		r = f.app.send(rt.method, rt.path, f.bob, rt.body)
		if denied(t, r) {
			t.Errorf("%s with %s %s: %d %s", name, rt.section, rt.action, r.status, r.text())
		}
	}
}

func denied(t *testing.T, r reply) bool {
	t.Helper()
	if r.status == 400 && code(t, r) == "validation.invalid_body" {
		return true
	}
	return r.status == 403 && code(t, r) == "auth.forbidden"
}

func applies(section, action string) bool {
	table := map[string][]string{
		"users": {"create", "read", "update"}, "groups": {"create", "read", "update", "delete"}, "tokens": {"read", "delete"},
		"clusters": {"create", "read", "update", "delete"}, "environments": {"create", "update", "delete"},
		"link_kinds": {"create", "update", "delete"}, "secrets": {"create", "read", "update", "delete"}, "scans": {"read"},
	}
	for _, a := range table[section] {
		if a == action {
			return true
		}
	}
	return false
}

func asStrings(v any) []string {
	s, _ := v.([]string)
	return s
}

func TestSectionRightsCannotRaiseRights(t *testing.T) {
	t.Parallel()
	f := newRightsFixture(t)
	f.grant(t, obj{"users": []string{"create", "read", "update"}, "groups": []string{"read", "update", "delete"}, "tokens": []string{"read", "delete"}})
	carol := f.app.user("carol@example.com", false)
	root := scalar[string](t, f.app.db, "SELECT id::text FROM users WHERE email = 'root@example.com'")
	support := idOf(createGroup(t, f.app, f.admin, "Support"))
	forbidden := func(r reply, what string) {
		t.Helper()
		eq(t, r.status, 403, what, " ", r.text())
		eq(t, code(t, r), any("auth.forbidden"), what)
	}

	forbidden(f.app.send("POST", "/api/v1/users", f.bob, obj{"email": "x@example.com", "display_name": "X", "password": "long-enough-1", "is_superadmin": true}), "create superadmin")
	eq(t, f.app.send("POST", "/api/v1/users", f.bob, obj{"email": "y@example.com", "display_name": "Y", "password": "long-enough-1"}).status, 201)
	forbidden(f.app.send("PATCH", "/api/v1/users/"+carol.ID.String(), f.bob, obj{"is_superadmin": true}), "set flag")
	forbidden(f.app.send("PATCH", "/api/v1/users/"+root, f.bob, obj{"status": "disabled"}), "disable superadmin")
	forbidden(f.app.send("POST", "/api/v1/users/"+root+"/password", f.bob, obj{"password": "long-enough-1"}), "reset superadmin")
	forbidden(f.app.send("PATCH", "/api/v1/users/"+f.bobID, f.bob, obj{"display_name": "Me"}), "self")
	eq(t, f.app.send("PATCH", "/api/v1/users/"+carol.ID.String(), f.bob, obj{"display_name": "Carol C"}).status, 200)

	forbidden(f.app.call("PUT", "/api/v1/groups/"+support+"/members/"+f.bobID, f.bob), "self into group")
	eq(t, f.app.call("PUT", "/api/v1/groups/"+support+"/members/"+carol.ID.String(), f.bob).status, 204)
	forbidden(f.app.call("PUT", "/api/v1/groups/"+f.group+"/members/"+carol.ID.String(), f.bob), "member of protected")
	forbidden(f.app.send("PATCH", "/api/v1/groups/"+f.group, f.bob, obj{"name": "Mine"}), "rename protected")
	forbidden(f.app.send("PUT", "/api/v1/groups/"+f.group+"/contacts", f.bob, obj{"contacts": []any{}, "on_call_url": nil}), "contacts protected")
	forbidden(f.app.call("DELETE", "/api/v1/groups/"+f.group, f.bob), "delete protected")

	execSQL(t, f.app.db, "INSERT INTO group_permissions (group_id, section, actions) VALUES ($1, 'scans', ARRAY['read'])", support)
	forbidden(f.app.send("PATCH", "/api/v1/users/"+carol.ID.String(), f.bob, obj{"display_name": "C"}), "user with rights")
	carolSession := f.app.session("carol@example.com", password)
	tokenID := idOf(f.app.issue(carolSession, obj{"name": "c", "scopes": []string{"read"}, "expires_in_days": 5}).json(t))
	forbidden(f.app.call("DELETE", "/api/v1/tokens/"+tokenID, f.bob), "token of user with rights")
	rootToken := idOf(f.app.issue(f.admin, obj{"name": "r", "scopes": []string{"read"}, "expires_in_days": 5}).json(t))
	forbidden(f.app.call("DELETE", "/api/v1/tokens/"+rootToken, f.bob), "token of superadmin")
	execSQL(t, f.app.db, "DELETE FROM group_permissions WHERE group_id = $1", support)
	eq(t, f.app.call("DELETE", "/api/v1/tokens/"+tokenID, f.bob).status, 204)
	eq(t, f.app.call("DELETE", "/api/v1/groups/"+support, f.bob).status, 204)
}

func TestRightsInAuthMeAndTokens(t *testing.T) {
	t.Parallel()
	f := newRightsFixture(t)
	other := idOf(createGroup(t, f.app, f.admin, "Readers"))
	eq(t, f.app.call("PUT", "/api/v1/groups/"+other+"/members/"+f.bobID, f.admin).status, 204)
	f.grant(t, obj{"clusters": []string{"read"}})
	eq(t, f.app.send("PUT", "/api/v1/groups/"+other+"/permissions", f.admin, obj{"permissions": obj{"clusters": []string{"update"}, "scans": []string{"read"}}}).status, 200)
	eq(t, jsonText(f.app.get("/api/v1/auth/me", f.bob).json(t)["rights"]), `{"clusters":["read","update"],"scans":["read"]}`)
	f.app.user("carol@example.com", false)
	eq(t, jsonText(f.app.get("/api/v1/auth/me", f.app.session("carol@example.com", password)).json(t)["rights"]), `{}`)
	full := f.app.get("/api/v1/auth/me", f.admin).json(t)["rights"].(obj)
	eq(t, len(full), 12)
	eq(t, jsonText(full["environments"]), `["create","update","delete"]`)

	read := f.app.pat(f.bob, "clusters:read")
	eq(t, f.app.bearer("GET", "/api/v1/clusters", read, nil).status, 200)
	r := f.app.bearer("PATCH", "/api/v1/clusters/"+uuid.Must(uuid.NewV7()).String(), read, obj{})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	eq(t, jsonText(f.app.bearer("GET", "/api/v1/auth/me", read, nil).json(t)["rights"]), `{"clusters":["read"]}`)
	write := f.app.pat(f.bob, "clusters:write", "settings:read")
	eq(t, f.app.bearer("GET", "/api/v1/knowledge/scans", write, nil).status, 200)
	r = f.app.bearer("GET", "/api/v1/secrets", write, nil)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	for _, scopes := range [][]string{{"settings:write"}, {"users:read"}, {"catalog.root"}} {
		r := f.app.issue(f.bob, obj{"name": "x", "scopes": scopes, "expires_in_days": 5})
		eq(t, r.status, 400, scopes)
		eq(t, code(t, r), any("validation.invalid_scopes"))
	}
	f.grant(t, obj{})
	r = f.app.bearer("GET", "/api/v1/clusters", read, nil)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	eq(t, f.app.send("PATCH", "/api/v1/users/"+f.bobID, f.admin, obj{"status": "disabled"}).status, 200)
	eq(t, f.app.bearer("GET", "/api/v1/knowledge/scans", write, nil).status, 401)
}

func TestProviderMembershipCarriesGroupRights(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t, "OAUTH_CORP_GROUP_MAP", "ops-team=>Ops")
	admin := app.admin()
	app.user("bob@example.com", false)
	ops := idOf(createGroup(t, app, admin, "Ops"))
	eq(t, app.send("PUT", "/api/v1/groups/"+ops+"/permissions", admin, obj{"permissions": obj{"clusters": []string{"read"}}}).status, 200)

	person := oidcfake.Person{Subject: "s-bob", Email: "bob@example.com", EmailVerified: true, Groups: []string{"ops-team"}}
	idp.SignIn(person)
	bob := cookieNamed(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", ""), "registry_session")
	eq(t, app.get("/api/v1/clusters", bob).status, 200)

	person.Groups = nil
	idp.SignIn(person)
	bob = cookieNamed(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", ""), "registry_session")
	r := app.get("/api/v1/clusters", bob)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
}

func TestSkillsRightIsASection(t *testing.T) {
	t.Parallel()
	f := newRightsFixture(t)
	r := f.grant(t, obj{"skills": []string{"delete", "create"}})
	eq(t, jsonText(r.json(t)), `{"permissions":{"skills":["create","delete"]}}`)
	r = f.app.send("PUT", "/api/v1/groups/"+f.group+"/permissions", f.admin, obj{"permissions": obj{"skills": []string{"read"}}})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_permissions"))
}

func TestMCPRightIsASection(t *testing.T) {
	t.Parallel()
	f := newRightsFixture(t)
	r := f.grant(t, obj{"mcp": []string{"update", "create"}})
	eq(t, jsonText(r.json(t)), `{"permissions":{"mcp":["create","update"]}}`)
	r = f.app.send("PUT", "/api/v1/groups/"+f.group+"/permissions", f.admin, obj{"permissions": obj{"mcp": []string{"read"}}})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_permissions"))
}
