package tests

import "testing"

type visibilityWorld struct {
	a                        *testApp
	admin, ann, ed, bob      string
	acme, globex             string
	jira, wiki, erp, tracker string
}

func newVisibilityWorld(t *testing.T) visibilityWorld {
	t.Helper()
	a := startGateway(t)
	w := visibilityWorld{a: a, admin: a.admin()}
	w.acme = a.nodeID(w.admin, "organization", "", "acme")
	w.globex = a.nodeID(w.admin, "organization", "", "globex")
	for email, role := range map[string]string{"ann@example.com": "admin", "ed@example.com": "editor", "bob@example.com": "viewer"} {
		a.user(email, false)
		a.grant(w.admin, w.acme, "user", email, role)
	}
	w.ann, w.ed, w.bob = a.session("ann@example.com", password), a.session("ed@example.com", password), a.session("bob@example.com", password)
	w.jira = idOf(a.mcpServer(w.admin, obj{"organization_id": w.acme, "slug": "jira", "status": "published"}))
	w.tracker = idOf(a.mcpServer(w.admin, obj{"organization_id": w.acme, "slug": "tracker", "status": "published"}))
	w.wiki = idOf(a.mcpServer(w.admin, obj{"slug": "wiki", "status": "published"}))
	w.erp = idOf(a.mcpServer(w.admin, obj{"organization_id": w.globex, "slug": "erp", "status": "published"}))
	return w
}

func TestMCPServerAccessListsAreManagedByAdmins(t *testing.T) {
	t.Parallel()
	w := newVisibilityWorld(t)
	a := w.a
	path := mcpServerPath(w.jira) + "/access"
	eq(t, jsonText(a.get(path, w.ed).json(t)), `{"can_manage":false,"everyone":true,"groups":[],"users":[]}`)
	eq(t, a.get(path, w.bob).status, 403)
	eq(t, a.send("PUT", path, w.ed, obj{"everyone": false}).status, 403)

	support := idOf(createGroup(t, a, w.admin, "Support"))
	other := a.send("POST", orgGroupsPath(w.globex), w.admin, obj{"name": "Globex ops"})
	eq(t, other.status, 201, other.text())
	r := a.send("PUT", path, w.ann, obj{"everyone": false, "groups": []string{support}})
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "groups", 0, "name"), any("Support"))
	eq(t, r.json(t)["everyone"], any(false))
	eq(t, code(t, a.send("PUT", path, w.ann, obj{"everyone": false, "groups": []string{idOf(other.json(t))}})), any("validation.invalid_mcp_access"))
	eq(t, code(t, a.send("PUT", path, w.ann, obj{"everyone": true, "users": []string{w.jira}})), any("validation.invalid_mcp_access"))
	eq(t, a.send("PUT", mcpServerPath(w.wiki)+"/access", w.ann, obj{"everyone": false}).status, 403)
}

func TestMCPOrganizationSetsAreManagedByAdmins(t *testing.T) {
	t.Parallel()
	w := newVisibilityWorld(t)
	a := w.a
	path := mcpPath + "/organizations/" + w.acme + "/servers"
	got := a.get(path, w.bob).json(t)
	eq(t, got["mode"], any("all"))
	eq(t, got["can_manage"], any(false))
	eq(t, a.send("PUT", path, w.ed, obj{"mode": "selected", "servers": []string{w.jira}}).status, 403)
	r := a.send("PUT", path, w.ann, obj{"mode": "selected", "servers": []string{w.jira, w.wiki}})
	eq(t, r.status, 200, r.text())
	eq(t, fieldsOf(t, obj{"items": r.json(t)["servers"]}, "slug"), "jira,wiki")
	eq(t, code(t, a.send("PUT", path, w.ann, obj{"mode": "selected", "servers": []string{w.erp}})), any("validation.invalid_mcp_servers"))
	eq(t, code(t, a.send("PUT", path, w.ann, obj{"mode": "some"})), any("validation.invalid_mcp_servers"))
}

func TestMCPVisibilitySummaryShowsWhatOneManages(t *testing.T) {
	t.Parallel()
	w := newVisibilityWorld(t)
	a := w.a
	eq(t, a.send("PUT", mcpPath+"/organizations/"+w.acme+"/servers", w.ann, obj{"mode": "selected", "servers": []string{w.wiki}}).status, 200)
	got := a.get(mcpPath+"/visibility", w.ann).json(t)
	eq(t, len(list(t, got, "organizations")), 1)
	eq(t, at(got, "organizations", 0, "mode"), any("selected"))
	eq(t, fieldsOf(t, obj{"items": got["servers"]}, "slug"), "jira,tracker,wiki")
	rows := list(t, got, "servers")
	eq(t, rows[0].(obj)["can_manage"], any(true))
	eq(t, len(rows[0].(obj)["in"].([]any)), 0)
	eq(t, rows[2].(obj)["can_manage"], any(false))
	eq(t, rows[2].(obj)["in"].([]any)[0], any(w.acme))
	eq(t, a.get(mcpPath+"/visibility", w.bob).status, 403)
	all := a.get(mcpPath+"/visibility", w.admin).json(t)
	eq(t, len(list(t, all, "organizations")), 2)
	eq(t, len(list(t, all, "servers")), 4)
}
