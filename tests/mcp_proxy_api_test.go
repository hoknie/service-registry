package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPProxySettingsAreInherited(t *testing.T) {
	t.Parallel()
	a := startGateway(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	a.user("ann@example.com", false)
	a.user("ed@example.com", false)
	a.grant(admin, acme, "user", "ann@example.com", "admin")
	a.grant(admin, acme, "user", "ed@example.com", "editor")
	ann, ed := a.session("ann@example.com", password), a.session("ed@example.com", password)
	team := a.mcpFolder(admin, obj{"organization_id": acme, "slug": "team", "name": "Team"})
	jira := a.mcpServer(admin, obj{"folder_id": idOf(team), "slug": "jira", "status": "published"})
	wiki := a.mcpServer(admin, obj{"folder_id": idOf(team), "slug": "wiki", "status": "published"})

	got := a.get(mcpServerPath(idOf(jira))+"/proxy", ed).json(t)
	eq(t, jsonText(got), `{"effective":{"credentials":null,"mode":"direct","no_proxy":null,"url":null},"from":null,"own":null}`)

	org := mcpPath + "/organizations/" + acme + "/proxy"
	eq(t, a.send("PUT", org, ed, obj{"mode": "proxy", "url": "http://proxy.acme.internal:3128"}).status, 403)
	secret := a.secret(admin, acme, obj{"name": "proxy", "value": "svc:pw"})
	r := a.send("PUT", org, ann, obj{"mode": "proxy", "url": "http://proxy.acme.internal:3128", "no_proxy": "internal.example",
		"credentials": obj{"secret_id": secret}})
	eq(t, r.status, 200, r.text())
	got = a.get(mcpServerPath(idOf(jira))+"/proxy", ed).json(t)
	eq(t, got["own"], nil)
	eq(t, at(got, "effective", "url"), any("http://proxy.acme.internal:3128"))
	eq(t, at(got, "effective", "credentials", "secret", "name"), any("proxy"))
	eq(t, at(got, "from", "kind"), any("organization"))
	eq(t, at(got, "from", "id"), any(acme))

	eq(t, a.send("PUT", mcpServerPath(idOf(wiki))+"/proxy", ed, obj{"mode": "direct"}).status, 200)
	got = a.get(mcpServerPath(idOf(wiki))+"/proxy", ed).json(t)
	eq(t, at(got, "effective", "mode"), any("direct"))
	eq(t, at(got, "from", "kind"), any("server"))

	eq(t, a.send("PUT", mcpPath+"/folders/"+idOf(team)+"/proxy", ed, obj{"mode": "environment"}).status, 200)
	eq(t, at(a.get(mcpServerPath(idOf(jira))+"/proxy", ed).json(t), "from", "kind"), any("folder"))
	eq(t, a.call("DELETE", mcpPath+"/folders/"+idOf(team)+"/proxy", ed).status, 204)
	eq(t, at(a.get(mcpServerPath(idOf(jira))+"/proxy", ed).json(t), "from", "kind"), any("organization"))

	r = a.send("PUT", org, ann, obj{"mode": "proxy", "url": "http://user:pass@proxy:3128"})
	eq(t, code(t, r), any("validation.invalid_mcp_proxy"))
	eq(t, code(t, a.send("PUT", org, ann, obj{"mode": "direct", "url": "http://proxy:3128"})), any("validation.invalid_mcp_proxy"))
}

func TestMCPProxyDefaultFollowsTheEnvironmentSwitch(t *testing.T) {
	t.Parallel()
	a := startGateway(t, "MCP_GATEWAY_USE_PROXY", "true")
	admin := a.admin()
	srv := a.mcpServer(admin, obj{"slug": "jira"})
	eq(t, at(a.get(mcpServerPath(idOf(srv))+"/proxy", admin).json(t), "effective", "mode"), any("environment"))
}

func TestMCPServersConnectThroughTheirProxy(t *testing.T) {
	t.Parallel()
	a := startGateway(t)
	admin := a.admin()
	seen := make(chan string, 4)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Host
		w.WriteHeader(http.StatusProxyAuthRequired)
	}))
	defer proxy.Close()
	srv := a.mcpServer(admin, obj{"slug": "jira", "url": "http://mcp.jira.test/mcp", "status": "published"})
	eq(t, a.send("PUT", mcpServerPath(idOf(srv))+"/proxy", admin, obj{"mode": "proxy", "url": proxy.URL}).status, 200)
	r := a.send("POST", mcpServerPath(idOf(srv))+"/check", admin, nil)
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "last_check", "code"), any("gateway.proxy"))
	eq(t, <-seen, "mcp.jira.test")
}

func TestDirectMCPConnectionsFollowTheAddressPolicy(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	srv := a.mcpServer(admin, obj{"slug": "internal", "url": "http://127.0.0.1:9/mcp", "status": "published"})
	r := a.send("POST", mcpServerPath(idOf(srv))+"/check", admin, nil)
	eq(t, at(r.json(t), "last_check", "code"), any("gateway.blocked"))
}
