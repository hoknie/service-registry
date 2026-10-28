package tests

import (
	"strings"
	"testing"
)

func TestMCPFoldersInheritAndMoveScopes(t *testing.T) {
	t.Parallel()
	a := startGateway(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	a.user("ed@example.com", false)
	a.grant(admin, acme, "user", "ed@example.com", "editor")
	ed := a.session("ed@example.com", password)

	team := a.mcpFolder(ed, obj{"organization_id": acme, "slug": "team", "name": "Team"})
	tracker := a.mcpFolder(ed, obj{"parent_id": idOf(team), "organization_id": nil, "slug": "tracker", "name": "Tracker"})
	eq(t, tracker["organization_id"], any(acme))
	eq(t, tracker["path"], any("team/tracker"))
	eq(t, a.send("POST", mcpPath+"/folders", ed, obj{"slug": "global", "name": "Global"}).status, 403)

	shared := a.mcpFolder(admin, obj{"slug": "shared", "name": "Shared"})
	srv := a.mcpServer(admin, obj{"folder_id": idOf(shared), "slug": "jira", "status": "published"})
	eq(t, srv["organization_id"], nil)
	r := a.send("PATCH", mcpPath+"/folders/"+idOf(shared), admin, obj{"parent_id": idOf(team)})
	eq(t, r.status, 200, r.text())
	eq(t, a.get(mcpServerPath(idOf(srv)), admin).json(t)["organization_id"], any(acme))
	eq(t, code(t, a.send("PATCH", mcpPath+"/folders/"+idOf(team), admin, obj{"parent_id": idOf(tracker)})), any("conflict.mcp_folder_cycle"))
	eq(t, code(t, a.call("DELETE", mcpPath+"/folders/"+idOf(team), admin)), any("conflict.mcp_folder_not_empty"))
	eq(t, a.call("DELETE", mcpPath+"/folders/"+idOf(tracker), ed).status, 204)
}

func TestMCPServersFollowTheRightsOfTheirScope(t *testing.T) {
	t.Parallel()
	a := startGateway(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	globex := a.nodeID(admin, "organization", "", "globex")
	a.user("bob@example.com", false)
	a.user("ed@example.com", false)
	a.grant(admin, acme, "user", "bob@example.com", "viewer")
	a.grant(admin, acme, "user", "ed@example.com", "editor")
	bob, ed := a.session("bob@example.com", password), a.session("ed@example.com", password)
	secret := a.secret(admin, acme, obj{"name": "jira-token", "value": "s3cret-jira-token"})

	r := a.send("POST", mcpPath+"/servers", ed, obj{"organization_id": acme, "slug": "jira", "title": "Jira",
		"url": "https://mcp.jira.example/mcp", "auth": obj{"kind": "bearer", "credentials": obj{"secret_id": secret}}})
	eq(t, r.status, 201, r.text())
	jira := r.json(t)
	eq(t, jira["status"], any("draft"))
	eq(t, jira["state"], any("unknown"))
	eq(t, jira["author"], any("ed"))
	eq(t, at(jira, "auth", "credentials", "secret", "name"), any("jira-token"))
	eq(t, strings.Contains(r.text(), "s3cret"), false)
	eq(t, a.get(mcpServerPath(idOf(jira)), bob).status, 404, "a draft is hidden from readers")

	eq(t, a.send("PATCH", mcpServerPath(idOf(jira)), ed, obj{"status": "published", "author": "someone"}).status, 200)
	seen := a.get(mcpServerPath(idOf(jira)), bob).json(t)
	eq(t, seen["can_write"], any(false))
	eq(t, seen["author"], any("ed"))
	eq(t, seen["url"], nil)
	eq(t, jsonText(seen["auth"]), `{"kind":"bearer"}`)
	eq(t, a.send("PATCH", mcpServerPath(idOf(jira)), bob, obj{"title": "X"}).status, 403)
	eq(t, a.send("PATCH", mcpServerPath(idOf(jira)), ed, obj{"slug": "jira2"}).json(t)["code"], any("validation.mcp_server_slug_fixed"))

	a.mcpServer(admin, obj{"organization_id": globex, "slug": "erp", "status": "published"})
	eq(t, fieldsOf(t, a.get(mcpPath+"/servers", bob).json(t), "slug"), "jira")
	a.user("eve@example.com", false)
	eve := a.session("eve@example.com", password)
	eq(t, a.get(mcpServerPath(idOf(jira)), eve).status, 404)

	r = a.send("POST", mcpPath+"/servers", admin, obj{"slug": "jira", "title": "Again", "url": "https://x.example/mcp"})
	eq(t, code(t, r), any("conflict.mcp_server_slug_taken"))
	r = a.send("POST", mcpPath+"/servers", admin, obj{"slug": "glob", "title": "G", "url": "https://x.example/mcp",
		"auth": obj{"kind": "bearer", "credentials": obj{"secret_id": secret}}})
	eq(t, code(t, r), any("validation.secret_not_available"))
	r = a.send("POST", mcpPath+"/servers", admin, obj{"slug": "Bad", "title": "", "url": "ftp://x"})
	eq(t, code(t, r), any("validation.invalid_mcp_server"))
	eq(t, len(list(t, r.json(t), "fields")), 3)

	secrets := a.get(secretsPath(acme), admin).json(t)
	eq(t, at(secrets, "items", 0, "used_by"), any(float64(1)))
}

func TestMCPServerListFilters(t *testing.T) {
	t.Parallel()
	a := startGateway(t)
	admin := a.admin()
	a.mcpServer(admin, obj{"slug": "jira", "tags": []string{"tracker"}, "status": "published"})
	a.mcpServer(admin, obj{"slug": "wiki", "tags": []string{"docs"}, "status": "published"})
	a.mcpServer(admin, obj{"slug": "youtrack", "tags": []string{"tracker"}, "status": "disabled"})
	eq(t, fieldsOf(t, a.get(mcpPath+"/servers?tag=tracker", admin).json(t), "slug"), "jira,youtrack")
	eq(t, fieldsOf(t, a.get(mcpPath+"/servers?tag=tracker&state=unknown", admin).json(t), "slug"), "jira")
	eq(t, fieldsOf(t, a.get(mcpPath+"/servers?q=WIK", admin).json(t), "slug"), "wiki")
	page := a.get(mcpPath+"/servers?limit=2", admin).json(t)
	eq(t, fieldsOf(t, page, "slug"), "jira,wiki")
	next := a.get(mcpPath+"/servers?limit=2&cursor="+page["next_cursor"].(string), admin).json(t)
	eq(t, fieldsOf(t, next, "slug"), "youtrack")
	eq(t, code(t, a.get(mcpPath+"/servers?state=broken", admin)), any("validation.invalid_filter"))
}

func TestGlobalMCPServersNeedTheMCPRightAndScope(t *testing.T) {
	t.Parallel()
	f := newRightsFixture(t)
	f.grant(t, obj{"mcp": []string{"create", "update"}})
	r := f.app.send("POST", mcpPath+"/servers", f.bob, obj{"slug": "jira", "title": "Jira", "url": "https://x.example/mcp",
		"status": "published"})
	eq(t, r.status, 201, r.text())
	id := idOf(r.json(t))
	eq(t, f.app.call("DELETE", mcpServerPath(id), f.bob).status, 403)
	write := f.app.pat(f.bob, "read", "write")
	eq(t, code(t, f.app.bearer("PATCH", mcpServerPath(id), write, obj{"title": "J"})), any("auth.insufficient_scope"))
	scoped := f.app.pat(f.bob, "write", "mcp_servers:write")
	eq(t, f.app.bearer("PATCH", mcpServerPath(id), scoped, obj{"title": "J"}).status, 200)
}

func TestMCPSpaceDescriptionOfAnOrganization(t *testing.T) {
	t.Parallel()
	a := startGateway(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	a.user("bob@example.com", false)
	a.user("ed@example.com", false)
	a.grant(admin, acme, "user", "bob@example.com", "viewer")
	a.grant(admin, acme, "user", "ed@example.com", "editor")
	bob, ed := a.session("bob@example.com", password), a.session("ed@example.com", password)
	path := mcpPath + "/organizations/" + acme
	r := a.send("PUT", path, ed, obj{"description": " Серверы команд acme "})
	eq(t, r.status, 200, r.text())
	got := a.get(path, bob).json(t)
	eq(t, got["description"], any("Серверы команд acme"))
	eq(t, got["can_write"], any(false))
	eq(t, a.send("PUT", path, bob, obj{"description": "x"}).status, 403)
	eq(t, code(t, a.send("PUT", path, ed, obj{"description": strings.Repeat("a", 2001)})), any("validation.invalid_mcp_space"))
}
