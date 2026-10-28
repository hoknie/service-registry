package tests

import (
	"context"
	"slices"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const builtinPath = "/api/v1/mcp/builtin"

func toolNames(t *testing.T, s *sdk.ClientSession) []string {
	t.Helper()
	res, err := s.ListTools(context.Background(), nil)
	must(t, err)
	var out []string
	for _, tool := range res.Tools {
		out = append(out, tool.Name)
	}
	return out
}

func TestBuiltinServerListsAndDisablesItsTools(t *testing.T) {
	t.Parallel()
	a := startGateway(t, "PUBLIC_URL", "https://registry.example")
	admin := a.admin()
	a.user("bob@example.com", false)
	bob := a.session("bob@example.com", password)

	r := a.get(builtinPath, bob)
	eq(t, r.status, 200, r.text())
	got := r.json(t)
	eq(t, got["slug"], any("svc-registry"))
	eq(t, got["builtin"], any(true))
	eq(t, got["can_write"], any(false))
	eq(t, got["endpoint"], any("https://registry.example/api/mcp"))
	names := fieldsOf(t, obj{"items": got["tools"]}, "name")
	for _, want := range []string{"list_projects", "search_docs", "get_skill", "get_guide", "list_mcp_servers"} {
		contains(t, names, want)
	}
	eq(t, strings.Contains(names, "__"), false)
	eq(t, at(got, "tools", 0, "enabled"), any(true))
	eq(t, at(got, "tools", 0, "input_schema", "type"), any("object"))

	eq(t, code(t, a.send("PUT", builtinPath+"/tools/disabled", bob, obj{"tools": []string{"search_docs"}})), any("auth.forbidden"))
	eq(t, code(t, a.send("PUT", builtinPath+"/tools/disabled", admin, obj{"tools": []string{"Bad-Name"}})), any("validation.invalid_builtin_tools"))
	eq(t, code(t, a.send("PUT", builtinPath+"/tools/disabled", admin, obj{"tools": []string{"get_doc", "get_doc"}})), any("validation.invalid_builtin_tools"))
	token := a.pat(admin, "write")
	eq(t, code(t, a.bearer("PUT", builtinPath+"/tools/disabled", token, obj{"tools": []string{"search_docs"}})), any("auth.insufficient_scope"))

	s := a.mcpSession(a.pat(bob, "mcp"))
	before := toolNames(t, s)
	r = a.send("PUT", builtinPath+"/tools/disabled", admin, obj{"tools": []string{"search_docs", "not_there"}})
	eq(t, r.status, 200, r.text())
	for _, tool := range list(t, r.json(t), "tools") {
		m := tool.(map[string]any)
		eq(t, m["enabled"], any(m["name"] != "search_docs"), m["name"])
	}
	after := toolNames(t, s)
	eq(t, len(after), len(before)-1)
	eq(t, slices.Contains(after, "search_docs"), false)
	_, err := s.CallTool(context.Background(), &sdk.CallToolParams{Name: "search_docs", Arguments: map[string]any{"query": "x"}})
	eq(t, err != nil, true, "a disabled tool is unknown")

	eq(t, a.send("PUT", builtinPath+"/tools/disabled", admin, obj{"tools": []string{}}).status, 200)
	eq(t, len(toolNames(t, s)), len(before))
	contains(t, execErr(t, a.db, "INSERT INTO mcp_builtin_settings (id) VALUES (false)"), "mcp_builtin_settings_id_check")
}
