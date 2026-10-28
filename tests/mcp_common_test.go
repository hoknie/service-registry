package tests

import (
	"context"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"svc-registry/pkg/testing/mcpfake"
)

const mcpPath = "/api/v1/mcp"

func mcpServerPath(id string) string { return mcpPath + "/servers/" + id }

func startGateway(t testing.TB, extra ...string) *testApp {
	t.Helper()
	return startApp(t, append([]string{"MCP_GATEWAY_ALLOW_HOSTS", "127.0.0.0/8", "SECRETS_KEYS", secretsKey}, extra...)...)
}

func (a *testApp) mcpFolder(cookie string, body obj) obj {
	a.t.Helper()
	r := a.send("POST", mcpPath+"/folders", cookie, body)
	if r.status != 201 {
		a.t.Fatalf("mcp folder: %d %s", r.status, r.body)
	}
	return r.json(a.t)
}

func (a *testApp) mcpServer(cookie string, body obj) obj {
	a.t.Helper()
	if _, ok := body["title"]; !ok {
		body["title"] = strings.ToUpper(body["slug"].(string)[:1]) + body["slug"].(string)[1:]
	}
	if _, ok := body["url"]; !ok {
		body["url"] = "https://" + body["slug"].(string) + ".example.test/mcp"
	}
	r := a.send("POST", mcpPath+"/servers", cookie, body)
	if r.status != 201 {
		a.t.Fatalf("mcp server: %d %s", r.status, r.body)
	}
	return r.json(a.t)
}

func (a *testApp) liveMCPServer(cookie string, fake *mcpfake.Server, body obj) obj {
	a.t.Helper()
	body["url"] = fake.URL
	if _, ok := body["status"]; !ok {
		body["status"] = "published"
	}
	s := a.mcpServer(cookie, body)
	r := a.send("POST", mcpServerPath(idOf(s))+"/check", cookie, nil)
	if r.status != 200 {
		a.t.Fatalf("check: %d %s", r.status, r.body)
	}
	return r.json(a.t)
}

func gatewayToolNames(t *testing.T, s *sdk.ClientSession) string {
	t.Helper()
	res, err := s.ListTools(context.Background(), nil)
	must(t, err)
	var out []string
	for _, tool := range res.Tools {
		if strings.Contains(tool.Name, "__") {
			out = append(out, tool.Name)
		}
	}
	return strings.Join(out, ",")
}

func (a *testApp) tokenFor(cookie string, body obj) (string, string) {
	a.t.Helper()
	if _, ok := body["name"]; !ok {
		body["name"] = "agent"
	}
	if _, ok := body["scopes"]; !ok {
		body["scopes"] = []string{"mcp"}
	}
	body["expires_in_days"] = 30
	r := a.issue(cookie, body)
	if r.status != 201 {
		a.t.Fatalf("issue: %d %s", r.status, r.body)
	}
	return r.json(a.t)["secret"].(string), idOf(r.json(a.t))
}
