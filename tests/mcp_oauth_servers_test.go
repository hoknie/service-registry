package tests

import (
	"context"
	"net/url"
	"testing"

	"golang.org/x/oauth2"

	"svc-registry/pkg/testing/mcpfake"
)

func TestOAuthGrantsCarryTheirServers(t *testing.T) {
	t.Parallel()
	app := oauthApp(t, "MCP_GATEWAY_ALLOW_HOSTS", "127.0.0.0/8")
	admin := app.admin()
	fake := mcpfake.Start(t, mcpfake.Tool{Name: "search"})
	app.liveMCPServer(admin, fake, obj{"slug": "jira"})
	wiki := idOf(app.liveMCPServer(admin, fake, obj{"slug": "wiki"}))
	app.user("bob@example.com", false)
	bob := app.session("bob@example.com", password)

	p := app.startAuthorization(app.registerClient())
	req := app.get("/api/v1/oauth/requests/"+p.requestID, bob).json(t)
	eq(t, fieldsOf(t, obj{"items": req["mcp_servers"]}, "slug"), "jira,wiki")
	r := app.send("POST", "/api/v1/oauth/requests/"+p.requestID+"/approve", bob, obj{"nodes": nil, "mcp_servers": []string{wiki}})
	eq(t, r.status, 200, r.text())
	code := urlQuery(t, r.json(t)["redirect_to"].(string)).Get("code")
	tok, err := p.cfg.Exchange(context.Background(), code, oauth2.VerifierOption(p.verifier), oauth2.SetAuthURLParam("resource", oauthResource))
	must(t, err)
	session := app.mcpSession(tok.AccessToken)
	eq(t, gatewayToolNames(t, session), "wiki__search")

	apps := app.get("/api/v1/account/apps", bob).json(t)
	grant := at(apps, "items", 0, "id").(string)
	eq(t, at(apps, "items", 0, "mcp_servers", 0, "slug"), any("wiki"))
	r = app.send("PUT", "/api/v1/account/apps/"+grant+"/mcp-servers", bob, obj{"servers": nil})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["mcp_servers"], nil)
	eq(t, gatewayToolNames(t, session), "jira__search,wiki__search")
	eq(t, app.send("PUT", "/api/v1/account/apps/"+grant+"/mcp-servers", admin, obj{"servers": nil}).status, 404)
}

func urlQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	must(t, err)
	return u.Query()
}
