package tests

import (
	"context"
	"strings"
	"testing"

	"svc-registry/pkg/testing/mcpfake"
)

func (a *testApp) runMCPChecks() int {
	a.t.Helper()
	ctx := context.Background()
	ids, err := a.services.Gateway.Checks.ClaimChecks(ctx, 50, 60)
	must(a.t, err)
	for _, id := range ids {
		must(a.t, a.services.Gateway.Checks.RunCheck(ctx, id))
	}
	return len(ids)
}

func TestMCPServerChecksKeepASnapshotOfTools(t *testing.T) {
	t.Parallel()
	a := startGateway(t)
	admin := a.admin()
	fake := mcpfake.Start(t, mcpfake.Tool{Name: "search", Description: "Search issues", ReadOnly: true},
		mcpfake.Tool{Name: "delete_issue"}, mcpfake.Tool{Name: "has space"})
	fake.RequireHeader("Authorization", "Bearer s3cret-jira-token")
	fake.SetInstructions("Search first")
	secret := a.secret(admin, "", obj{"name": "jira", "value": "s3cret-jira-token"})
	jira := a.liveMCPServer(admin, fake, obj{"slug": "jira", "title": "Jira",
		"auth": obj{"kind": "bearer", "credentials": obj{"secret_id": secret}}})
	eq(t, jira["state"], any("ok"))
	eq(t, jira["tools_count"], any(float64(3)))
	eq(t, at(jira, "server_info", "name"), any("fake"))
	eq(t, at(jira, "server_info", "instructions"), any("Search first"))
	id := idOf(jira)
	tools := a.get(mcpServerPath(id)+"/tools", admin).json(t)
	eq(t, fieldsOf(t, tools, "gateway_name"), "jira__delete_issue,jira__has space,jira__search")
	eq(t, at(tools, "items", 1, "skipped"), any("name_invalid"))

	r := a.send("PUT", mcpServerPath(id)+"/tools/disabled", admin, obj{"tools": []string{"delete_issue"}})
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "items", 0, "enabled"), any(false))
	eq(t, at(r.json(t), "items", 2, "enabled"), any(true))

	eq(t, code(t, a.send("POST", mcpServerPath(id)+"/check", admin, nil)), any("gateway.check_rate_limited"))
	execSQL(t, a.db, "UPDATE mcp_server_checks SET checked_at = now() - interval '1 minute', next_check_at = now()")
	fake.Stop()
	eq(t, a.runMCPChecks(), 1)
	after := a.get(mcpServerPath(id), admin).json(t)
	eq(t, after["state"], any("failing"))
	eq(t, at(after, "last_check", "code"), any("gateway.unreachable"))
	eq(t, after["tools_count"], any(float64(3)))
	eq(t, a.runMCPChecks(), 0)
}

func TestMCPServerCheckReportsAWrongToken(t *testing.T) {
	t.Parallel()
	a := startGateway(t)
	admin := a.admin()
	fake := mcpfake.Start(t, mcpfake.Tool{Name: "search"})
	fake.RequireHeader("X-Api-Key", "right-key")
	secret := a.secret(admin, "", obj{"name": "key", "value": "wrong-key-value"})
	srv := a.liveMCPServer(admin, fake, obj{"slug": "wiki", "auth": obj{"kind": "header", "header": "X-Api-Key",
		"credentials": obj{"secret_id": secret}}})
	eq(t, srv["state"], any("unknown"))
	eq(t, at(srv, "last_check", "code"), any("gateway.unauthorized"))
	eq(t, strings.Contains(jsonText(srv), "wrong-key-value"), false)
}

func TestMCPServerChecksAreQueuedByChanges(t *testing.T) {
	t.Parallel()
	a := startGateway(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	fake := mcpfake.Start(t, mcpfake.Tool{Name: "search"})
	srv := a.mcpServer(admin, obj{"organization_id": acme, "slug": "jira", "url": fake.URL, "status": "published"})
	eq(t, a.runMCPChecks(), 1)
	eq(t, a.get(mcpServerPath(idOf(srv)), admin).json(t)["state"], any("ok"))
	eq(t, a.runMCPChecks(), 0)
	r := a.send("PUT", mcpPath+"/organizations/"+acme+"/proxy", admin, obj{"mode": "direct"})
	eq(t, r.status, 200, r.text())
	eq(t, a.runMCPChecks(), 1)
	eq(t, a.send("PATCH", mcpServerPath(idOf(srv)), admin, obj{"title": "Jira 2"}).status, 200)
	eq(t, a.runMCPChecks(), 0)
	eq(t, a.send("PATCH", mcpServerPath(idOf(srv)), admin, obj{"status": "disabled"}).status, 200)
	eq(t, a.runMCPChecks(), 0)
	eq(t, a.get(mcpServerPath(idOf(srv)), admin).json(t)["state"], any("disabled"))
}

func TestMCPServerIconsComeFromTheServer(t *testing.T) {
	t.Parallel()
	a := startGateway(t)
	admin := a.admin()
	fake := mcpfake.Start(t, mcpfake.Tool{Name: "search"})
	fake.SetFavicon(tinyPNG)
	srv := a.liveMCPServer(admin, fake, obj{"slug": "jira"})
	eq(t, srv["has_icon"], any(true))
	eq(t, srv["author"], any("root"))
	r := a.get(mcpServerPath(idOf(srv))+"/icon", admin)
	eq(t, r.status, 200)
	eq(t, r.header("Content-Type"), "image/png")
	contains(t, r.header("Content-Security-Policy"), "sandbox")
	r = a.send("PATCH", mcpServerPath(idOf(srv)), admin, obj{"icon": "bug"})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["icon"], any("bug"))
	eq(t, code(t, a.send("PATCH", mcpServerPath(idOf(srv)), admin, obj{"icon": "Bad Icon"})), any("validation.invalid_mcp_server"))
	plain := a.liveMCPServer(admin, mcpfake.Start(t, mcpfake.Tool{Name: "x"}), obj{"slug": "wiki"})
	eq(t, plain["has_icon"], any(false))
	eq(t, a.get(mcpServerPath(idOf(plain))+"/icon", admin).status, 404)
}
