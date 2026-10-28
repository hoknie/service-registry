package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"svc-registry/pkg/testing/mcpfake"
)

func TestGatewayToolsAreListedWithAPrefix(t *testing.T) {
	t.Parallel()
	a := startGateway(t)
	admin := a.admin()
	fake := mcpfake.Start(t, mcpfake.Tool{Name: "search", Description: "Search issues", ReadOnly: true}, mcpfake.Tool{Name: "create_issue"})
	a.liveMCPServer(admin, fake, obj{"slug": "jira", "title": "Jira"})
	a.mcpServer(admin, obj{"slug": "wiki", "status": "published"})
	s := a.mcpSession(a.pat(admin, "mcp"))
	res, err := s.ListTools(context.Background(), nil)
	must(t, err)
	eq(t, len(res.Tools), 23)
	byName := map[string]*sdk.Tool{}
	for _, tool := range res.Tools {
		byName[tool.Name] = tool
	}
	search := byName["jira__search"]
	eq(t, search != nil && byName["jira__create_issue"] != nil && byName["list_mcp_servers"] != nil, true)
	eq(t, strings.HasPrefix(search.Description, "[Jira] Search issues"), true)
	eq(t, search.Title, "search (Jira)")
	eq(t, search.Annotations != nil && search.Annotations.ReadOnlyHint, true)
	text, isErr := call(t, s, "list_mcp_servers", nil)
	eq(t, isErr, false)
	eq(t, strings.Contains(text, `"prefix": "jira__"`) && !strings.Contains(text, fake.URL), true, text)
	guide, _ := call(t, s, "get_guide", map[string]any{"topic": "gateway"})
	eq(t, strings.Contains(guide, "gateway.unreachable"), true)
	unknown, isErr := call(t, s, "get_guide", map[string]any{"topic": "billing"})
	eq(t, isErr, true)
	eq(t, unknown, "not_found")
}

func TestGatewayCallsReachTheServerWithItsSecret(t *testing.T) {
	t.Parallel()
	a := startGateway(t)
	admin := a.admin()
	fake := mcpfake.Start(t, mcpfake.Tool{Name: "search"})
	fake.RequireHeader("Authorization", "Bearer s3cret-jira-token")
	secret := a.secret(admin, "", obj{"name": "jira", "value": "s3cret-jira-token"})
	jira := a.liveMCPServer(admin, fake, obj{"slug": "jira", "auth": obj{"kind": "bearer", "credentials": obj{"secret_id": secret}}})
	wikiFake := mcpfake.Start(t, mcpfake.Tool{Name: "page"})
	wiki := a.liveMCPServer(admin, wikiFake, obj{"slug": "wiki"})
	token, _ := a.tokenFor(admin, obj{"name": "laptop"})
	s := a.mcpSession(token)

	text, isErr := call(t, s, "jira__search", map[string]any{"query": "bug"})
	eq(t, isErr, false)
	eq(t, text, `search:{"query":"bug"}`)
	calls := fake.Calls()
	eq(t, len(calls), 1)
	eq(t, calls[0].Tool, "search")
	eq(t, calls[0].Auth, "Bearer s3cret-jira-token")
	var args map[string]any
	must(t, json.Unmarshal(calls[0].Args, &args))
	eq(t, args["query"], any("bug"))

	log := a.get(mcpServerPath(idOf(jira))+"/calls", admin).json(t)
	eq(t, at(log, "items", 0, "tool"), any("search"))
	eq(t, at(log, "items", 0, "outcome"), any("ok"))
	eq(t, at(log, "items", 0, "via", "name"), any("laptop"))
	eq(t, at(log, "items", 0, "user", "email"), any("root@example.com"))
	eq(t, at(log, "items", 0, "user", "name") != any(""), true, jsonText(log))
	eq(t, at(log, "items", 0, "request"), any(`{"query":"bug"}`))
	eq(t, strings.Contains(fmt.Sprint(at(log, "items", 0, "response")), "search:"), true, jsonText(log))
	eq(t, at(log, "items", 0, "truncated"), any(false))
	eq(t, strings.Contains(jsonText(log), "s3cret-jira-token"), false)
	eq(t, a.get(mcpServerPath(idOf(jira)), admin).json(t)["calls"], any(float64(1)))

	r := a.send("PATCH", mcpServerPath(idOf(jira)), admin, obj{"log_payloads": false})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["log_payloads"], any(false))
	_, _ = call(t, s, "jira__search", map[string]any{"query": "secret plans"})
	log = a.get(mcpServerPath(idOf(jira))+"/calls", admin).json(t)
	eq(t, at(log, "items", 0, "request"), nil)
	eq(t, at(log, "items", 0, "response"), nil)
	eq(t, strings.Contains(jsonText(log), "secret plans"), false)

	limited, _ := a.tokenFor(admin, obj{"mcp_servers": []string{idOf(wiki)}})
	text, isErr = call(t, a.mcpSession(limited), "jira__search", map[string]any{"query": "x"})
	eq(t, isErr, true)
	eq(t, text, "not_found")
	eq(t, len(fake.Calls()), 2)
}

func TestGatewayCallFailuresHaveCodes(t *testing.T) {
	t.Parallel()
	a := startGateway(t, "MCP_GATEWAY_CALL_TIMEOUT_SECS", "1")
	admin := a.admin()
	fake := mcpfake.Start(t, mcpfake.Tool{Name: "slow"})
	jira := a.liveMCPServer(admin, fake, obj{"slug": "jira"})
	fake.SetDelay(3 * time.Second)
	s := a.mcpSession(a.pat(admin, "mcp"))
	start := time.Now()
	text, isErr := call(t, s, "jira__slow", nil)
	eq(t, isErr, true)
	eq(t, strings.HasPrefix(text, "gateway.timeout"), true, text)
	eq(t, time.Since(start) < 3*time.Second, true)
	log := a.get(mcpServerPath(idOf(jira))+"/calls?outcome=failed", admin).json(t)
	eq(t, at(log, "items", 0, "code"), any("gateway.timeout"))
	eq(t, a.get(mcpServerPath(idOf(jira)), admin).json(t)["calls"], any(float64(0)))
}

func TestGatewayCanBeTurnedOff(t *testing.T) {
	t.Parallel()
	a := startApp(t, "MCP_GATEWAY_ENABLED", "false")
	admin := a.admin()
	eq(t, a.get(mcpPath+"/servers", admin).status, 404)
	eq(t, at(a.get("/api/v1/auth/me", admin).json(t), "features", "mcp_gateway"), any(false))
	s := a.mcpSession(a.pat(admin, "mcp"))
	res, err := s.ListTools(context.Background(), nil)
	must(t, err)
	for _, tool := range res.Tools {
		eq(t, tool.Name != "list_mcp_servers", true)
	}
	text, _ := call(t, s, "get_guide", map[string]any{"topic": "gateway"})
	eq(t, strings.Contains(text, "not_found"), true)
}
