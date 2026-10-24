package tests

import (
	"testing"

	"github.com/google/uuid"
)

func TestTokenWriteFollowsTheOwnerAction(t *testing.T) {
	t.Parallel()
	f := newRightsFixture(t)
	f.grant(t, obj{"clusters": []string{"read", "update"}})
	secret := f.app.pat(f.bob, "clusters:read", "clusters:write")
	id := uuid.Must(uuid.NewV7()).String()
	eq(t, f.app.bearer("PATCH", "/api/v1/clusters/"+id, secret, obj{"name": "x"}).status, 404)
	for _, r := range []reply{
		f.app.bearer("POST", "/api/v1/clusters", secret, obj{"name": "x", "mode": "agent"}),
		f.app.bearer("DELETE", "/api/v1/clusters/"+id, secret, nil),
	} {
		eq(t, r.status, 403, r.text())
		eq(t, code(t, r), any("auth.forbidden"))
	}
	read := f.app.pat(f.bob, "clusters:read")
	r := f.app.bearer("PATCH", "/api/v1/clusters/"+id, read, obj{"name": "x"})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
}

type stewardsFixture struct {
	rightsFixture
	acme, backend, team, carolID string
	dana                         string
}

func newStewardsFixture(t *testing.T) stewardsFixture {
	t.Helper()
	f := newRightsFixture(t)
	f.grant(t, obj{"groups": []string{"read", "update"}})
	acme := f.app.nodeID(f.admin, "organization", "", "acme")
	backend := f.app.nodeID(f.admin, "folder", acme, "backend")
	team := idOf(createGroup(t, f.app, f.admin, "Backend"))
	f.app.grant(f.admin, acme, "group", "Backend", "editor")
	f.app.grant(f.admin, backend, "group", "Backend", "viewer")
	f.app.user("dana@example.com", false)
	carol := f.app.user("carol@example.com", false)
	eq(t, f.app.send("PUT", ownersPath(acme), f.admin, obj{"owners": []obj{{"subject_kind": "user", "subject": "dana@example.com"}}}).status, 200)
	return stewardsFixture{rightsFixture: f, acme: acme, backend: backend, team: team, carolID: carol.ID.String(),
		dana: f.app.session("dana@example.com", password)}
}

func TestGroupWithCatalogRolesIsManagedByNodeOwners(t *testing.T) {
	t.Parallel()
	f := newStewardsFixture(t)
	r := f.app.call("PUT", "/api/v1/groups/"+f.team+"/members/"+f.carolID, f.bob)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	g := f.app.get("/api/v1/groups/"+f.team, f.bob).json(t)
	eq(t, g["members_lock"], any("bindings"))
	eq(t, g["can_manage_members"], any(false))
	eq(t, f.app.get("/api/v1/groups/"+f.group, f.admin).json(t)["members_lock"], any("rights"))

	eq(t, f.app.call("PUT", "/api/v1/account/managed-groups/"+f.team+"/members/"+f.carolID, f.dana).status, 204)
	eq(t, f.app.get("/api/v1/groups/"+f.team, f.admin).json(t)["member_count"], any(float64(1)))
	danaID := f.app.get("/api/v1/auth/me", f.dana).json(t)["id"].(string)
	eq(t, f.app.call("PUT", "/api/v1/account/managed-groups/"+f.team+"/members/"+danaID, f.dana).status, 403)
	eq(t, f.app.call("DELETE", "/api/v1/account/managed-groups/"+f.team+"/members/"+f.carolID, f.dana).status, 204)

	f.app.user("erin@example.com", false)
	r = f.app.send("PUT", ownersPath(f.backend), f.admin, obj{"owners": []obj{{"subject_kind": "user", "subject": "erin@example.com"}}})
	eq(t, r.status, 200, r.text())
	eq(t, f.app.call("PUT", "/api/v1/account/managed-groups/"+f.team+"/members/"+f.carolID, f.dana).status, 204)
	erin := f.app.session("erin@example.com", password)
	eq(t, f.app.call("DELETE", "/api/v1/account/managed-groups/"+f.team+"/members/"+f.carolID, erin).status, 403)
	eq(t, f.app.call("DELETE", "/api/v1/groups/"+f.team+"/members/"+f.carolID, f.admin).status, 204)
}

func TestOwnerGroupMembersManageGroupsAndCandidates(t *testing.T) {
	t.Parallel()
	f := newStewardsFixture(t)
	leads := idOf(createGroup(t, f.app, f.admin, "Leads"))
	danaID := f.app.get("/api/v1/auth/me", f.dana).json(t)["id"].(string)
	eq(t, f.app.call("PUT", "/api/v1/groups/"+leads+"/members/"+danaID, f.admin).status, 204)
	eq(t, f.app.send("PUT", ownersPath(f.acme), f.admin, obj{"owners": []obj{{"subject_kind": "group", "subject": "Leads"}}}).status, 200)
	f.app.grant(f.admin, f.acme, "group", "Ops", "viewer")

	list := f.app.get("/api/v1/account/managed-groups", f.dana).json(t)["items"].([]any)
	eq(t, len(list), 2)
	eq(t, at(list[0], "name"), any("Backend"))
	eq(t, len(at(list[0], "nodes").([]any)), 2)
	eq(t, at(list[1], "name"), any("Leads"))
	eq(t, at(list[1], "nodes", 0, "id"), any(f.acme))
	eq(t, len(f.app.get("/api/v1/account/managed-groups", f.bob).json(t)["items"].([]any)), 0)

	detail := f.app.get("/api/v1/account/managed-groups/"+f.team, f.dana)
	eq(t, detail.status, 200, detail.text())
	eq(t, detail.json(t)["can_manage_members"], any(true))
	cands := f.app.get("/api/v1/account/managed-groups/"+f.team+"/candidates?q=CAR", f.dana).json(t)["items"].([]any)
	eq(t, len(cands), 1)
	eq(t, at(cands[0], "email"), any("carol@example.com"))
	for _, path := range []string{"/api/v1/account/managed-groups/" + f.group, "/api/v1/account/managed-groups/" + f.group + "/candidates"} {
		eq(t, f.app.get(path, f.dana).status, 404, path)
	}
	eq(t, f.app.get("/api/v1/account/managed-groups/"+leads+"/candidates", f.bob).status, 404)
	eq(t, len(f.app.get("/api/v1/account/managed-groups/"+leads+"/candidates", f.dana).json(t)["items"].([]any)) > 0, true)
}

func TestCatalogRightOpensTheTopLevel(t *testing.T) {
	t.Parallel()
	f := newRightsFixture(t)
	r := f.app.createNode(f.bob, obj{"kind": "organization", "parent_id": nil, "slug": "beta", "name": "Beta"})
	eq(t, r.status, 403)
	f.grant(t, obj{"catalog": []string{"create"}})
	beta := f.app.nodeID(f.bob, "organization", "", "beta")
	eq(t, slugs(t, f.app.get("/api/v1/catalog/nodes", f.bob).json(t), "items"), "beta")
	r = f.app.call("DELETE", nodePath(beta), f.bob)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))

	whole := f.app.pat(f.bob, "write", "read", "catalog.root")
	eq(t, f.app.bearer("POST", "/api/v1/catalog/nodes", whole, obj{"kind": "organization", "slug": "gamma", "name": "Gamma"}).status, 201)
	scoped := f.app.secretOn(f.bob, []string{beta}, "write", "read", "catalog.root")
	eq(t, f.app.bearer("POST", "/api/v1/catalog/nodes", scoped, obj{"kind": "organization", "slug": "delta", "name": "Delta"}).status, 201)
	tokens := f.app.get("/api/v1/account/tokens", f.bob).json(t)["items"].([]any)
	found := false
	for _, tok := range tokens {
		if nodes, ok := at(tok, "nodes").([]any); ok && len(nodes) == 2 {
			found = true
		}
	}
	eq(t, found, true)

	f.grant(t, obj{"catalog": []string{"create", "delete"}})
	eq(t, f.app.call("DELETE", nodePath(beta), f.bob).status, 204)
	f.app.user("carol@example.com", false)
	r = f.app.issue(f.app.session("carol@example.com", password), obj{"name": "x", "scopes": []string{"write", "catalog.root"}, "expires_in_days": 5})
	eq(t, r.status, 400)
}
