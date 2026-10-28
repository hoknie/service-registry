package tests

import (
	"testing"

	"svc-registry/pkg/testing/mcpfake"
)

type setWorld struct {
	a                      *testApp
	admin, bob, carol, ed  string
	acme                   string
	jira, wiki, erp, draft string
}

func newSetWorld(t *testing.T) setWorld {
	t.Helper()
	a := startGateway(t)
	w := setWorld{a: a, admin: a.admin()}
	w.acme = a.nodeID(w.admin, "organization", "", "acme")
	globex := a.nodeID(w.admin, "organization", "", "globex")
	bob := a.user("bob@example.com", false)
	a.user("carol@example.com", false)
	a.user("ed@example.com", false)
	a.grant(w.admin, w.acme, "user", "bob@example.com", "viewer")
	a.grant(w.admin, w.acme, "user", "carol@example.com", "viewer")
	a.grant(w.admin, w.acme, "user", "ed@example.com", "editor")
	w.bob, w.carol, w.ed = a.session("bob@example.com", password), a.session("carol@example.com", password), a.session("ed@example.com", password)
	support := idOf(createGroup(t, a, w.admin, "Support"))
	eq(t, a.call("PUT", "/api/v1/groups/"+support+"/members/"+bob.ID.String(), w.admin).status, 204)
	fake := mcpfake.Start(t, mcpfake.Tool{Name: "search"})
	w.jira = idOf(a.liveMCPServer(w.admin, fake, obj{"organization_id": w.acme, "slug": "jira"}))
	w.wiki = idOf(a.liveMCPServer(w.admin, fake, obj{"slug": "wiki"}))
	w.erp = idOf(a.liveMCPServer(w.admin, fake, obj{"organization_id": globex, "slug": "erp"}))
	w.draft = idOf(a.liveMCPServer(w.admin, fake, obj{"organization_id": w.acme, "slug": "beta", "status": "draft"}))
	eq(t, a.send("PUT", mcpServerPath(w.jira)+"/access", w.admin, obj{"everyone": false, "groups": []string{support}}).status, 200)
	return w
}

func (w setWorld) tools(t *testing.T, cookie string, body obj) string {
	t.Helper()
	secret, _ := w.a.tokenFor(cookie, body)
	return gatewayToolNames(t, w.a.mcpSession(secret))
}

func TestGatewaySetIntersectsAccessOrganizationAndToken(t *testing.T) {
	t.Parallel()
	w := newSetWorld(t)
	eq(t, w.tools(t, w.bob, obj{}), "jira__search,wiki__search")
	eq(t, w.tools(t, w.carol, obj{}), "wiki__search")
	eq(t, w.tools(t, w.ed, obj{}), "beta__search,wiki__search")
	eq(t, w.tools(t, w.admin, obj{}), "beta__search,erp__search,wiki__search")

	eq(t, w.a.send("PUT", mcpPath+"/organizations/"+w.acme+"/servers", w.admin, obj{"mode": "selected", "servers": []string{w.jira}}).status, 200)
	eq(t, w.tools(t, w.bob, obj{"nodes": []string{w.acme}}), "jira__search")
	eq(t, w.tools(t, w.bob, obj{}), "jira__search,wiki__search")
	eq(t, w.tools(t, w.bob, obj{"mcp_servers": []string{w.wiki}}), "wiki__search")
	eq(t, w.tools(t, w.bob, obj{"mcp_servers": []string{}}), "")
}

func TestTokenServerListChangesWithoutReissue(t *testing.T) {
	t.Parallel()
	w := newSetWorld(t)
	a := w.a
	secret, id := a.tokenFor(w.bob, obj{})
	session := a.mcpSession(secret)
	eq(t, gatewayToolNames(t, session), "jira__search,wiki__search")
	r := a.send("PUT", "/api/v1/account/tokens/"+id+"/mcp-servers", w.bob, obj{"servers": []string{w.jira}})
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(r.json(t)["mcp_servers"]), `[{"id":"`+w.jira+`","slug":"jira","title":"Jira"}]`)
	eq(t, gatewayToolNames(t, session), "jira__search")
	listed := a.get("/api/v1/account/tokens", w.bob).json(t)
	eq(t, at(listed, "items", 0, "mcp_servers", 0, "slug"), any("jira"))

	eq(t, code(t, a.send("PUT", "/api/v1/account/tokens/"+id+"/mcp-servers", w.bob, obj{"servers": []string{w.erp}})), any("validation.invalid_mcp_servers"))
	eq(t, a.send("PUT", "/api/v1/account/tokens/"+id+"/mcp-servers", w.carol, obj{"servers": nil}).status, 404)
	byToken := a.bearer("PUT", "/api/v1/account/tokens/"+id+"/mcp-servers", a.pat(w.bob, "read", "write"), obj{"servers": nil})
	eq(t, code(t, byToken), any("auth.session_required"))
	r = a.send("PUT", "/api/v1/users/"+at(a.get("/api/v1/auth/me", w.bob).json(t), "id").(string)+"/tokens/"+id+"/mcp-servers",
		w.admin, obj{"servers": nil})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["mcp_servers"], nil)
	eq(t, gatewayToolNames(t, session), "jira__search,wiki__search")

	eq(t, a.call("DELETE", mcpServerPath(w.wiki), w.admin).status, 204)
	eq(t, a.send("PUT", "/api/v1/account/tokens/"+id+"/mcp-servers", w.bob, obj{"servers": []string{w.jira}}).status, 200)
	eq(t, a.call("DELETE", mcpServerPath(w.jira), w.admin).status, 204)
	for _, item := range list(t, a.get("/api/v1/account/tokens", w.bob).json(t), "items") {
		if item.(obj)["id"] == id {
			eq(t, jsonText(item.(obj)["mcp_servers"]), "[]")
		}
	}
	eq(t, gatewayToolNames(t, session), "")

	eq(t, a.call("DELETE", "/api/v1/account/tokens/"+id, w.bob).status, 204)
	eq(t, code(t, a.send("PUT", "/api/v1/account/tokens/"+id+"/mcp-servers", w.bob, obj{"servers": nil})), any("conflict.token_not_active"))
}

func TestServerListsNeedTheMCPScope(t *testing.T) {
	t.Parallel()
	w := newSetWorld(t)
	r := w.a.issue(w.bob, obj{"name": "x", "scopes": []string{"read"}, "expires_in_days": 30, "mcp_servers": []string{w.wiki}})
	eq(t, code(t, r), any("validation.invalid_mcp_servers"))
	r = w.a.issue(w.bob, obj{"name": "x", "scopes": []string{"mcp"}, "expires_in_days": 30, "mcp_servers": []string{w.erp}})
	eq(t, code(t, r), any("validation.invalid_mcp_servers"))
	r = w.a.issue(w.bob, obj{"name": "x", "scopes": []string{"mcp"}, "expires_in_days": 30})
	eq(t, r.json(t)["mcp_servers"], nil)
}
