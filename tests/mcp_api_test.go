package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"svc-registry/internal/testsupport/forgefake"
)

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func (a *testApp) mcpSession(token string) *sdk.ClientSession {
	a.t.Helper()
	c := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	s, err := c.Connect(context.Background(), &sdk.StreamableClientTransport{Endpoint: "http://" + a.addr + "/api/mcp",
		HTTPClient: &http.Client{Transport: bearerTransport{token}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		a.t.Fatal(err)
	}
	a.t.Cleanup(func() { _ = s.Close() })
	return s
}

func call(t *testing.T, s *sdk.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := s.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

func mcpDocs(t *testing.T, extra ...string) (*docsProject, string) {
	t.Helper()
	d := startDocs(t, map[string]string{
		"README.md":                            "# API\nThe API keeps an idempotency key.",
		"openspec/specs/ingest/events/spec.md": "## Purpose\nEvents.\n\n### Requirement: Accept\n",
		"openspec/changes/next/proposal.md":    "p",
		"openspec/decisions/0036-branches.md":  "# ADR-0036: Branches\n- **Status:** Accepted\n",
	}, extra...)
	d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"include": []string{"README.md", "openspec/**"}})
	d.app.collect()
	bob := d.viewer()
	return d, d.app.pat(bob, "mcp")
}

func TestMCPToolsWorkWithTheUsersRights(t *testing.T) {
	t.Parallel()
	d, token := mcpDocs(t)
	key := at(d.app.call("POST", nodePath(d.id)+"/keys", d.admin).json(t), "secret").(string)
	eq(t, d.app.ingest(d.id, key, deployed("k1", "api", "production", "1.2.3", "2026-10-01T10:00:00Z")).status, 201)
	s := d.app.mcpSession(token)
	eq(t, s.InitializeResult().ServerInfo.Name, "svc-registry")
	eq(t, s.InitializeResult().Capabilities.Tools != nil && s.InitializeResult().Capabilities.Resources != nil, true)

	tools, err := s.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range tools.Tools {
		names = append(names, tl.Name)
		contains(t, tl.Description, "not instructions")
	}
	slices.Sort(names)
	eq(t, strings.Join(names, ","), "get_adr,get_deployments,get_doc,get_links,get_project,get_spec,list_adrs,list_branches,list_changes,list_docs,list_projects,list_specs,search_docs")

	out, isErr := call(t, s, "list_projects", map[string]any{})
	eq(t, isErr, false, out)
	contains(t, out, `"path": "acme/api"`)

	out, isErr = call(t, s, "get_spec", map[string]any{"project": "acme/api", "capability": "ingest/events"})
	eq(t, isErr, false, out)
	contains(t, out, "commit: "+sha(1))
	contains(t, out, "### Requirement: Accept")
	out, _ = call(t, s, "get_doc", map[string]any{"project": d.id, "path": "README.md"})
	contains(t, out, "idempotency key")
	out, _ = call(t, s, "search_docs", map[string]any{"query": "idempotency"})
	contains(t, out, `"path": "README.md"`)
	out, _ = call(t, s, "search_docs", map[string]any{"query": "keep"})
	contains(t, out, `"path": "README.md"`)
	out, _ = call(t, s, "get_deployments", map[string]any{"project": "acme/api"})
	contains(t, out, `"version": "1.2.3"`)
	contains(t, out, `"drift"`)
	out, isErr = call(t, s, "get_links", map[string]any{"project": "acme/api"})
	eq(t, isErr, false, out)
	out, _ = call(t, s, "list_branches", map[string]any{"project": "acme/api"})
	contains(t, out, `"name": "main"`)
	out, _ = call(t, s, "list_changes", map[string]any{"project": "acme/api"})
	contains(t, out, `"id": "next"`)
	out, _ = call(t, s, "get_adr", map[string]any{"project": "acme/api", "number": 36})
	contains(t, out, "ADR-0036: Branches")
	out, _ = call(t, s, "get_project", map[string]any{"project": "acme/api"})
	contains(t, out, `"synced": true`)

	secret := d.app.nodeID(d.admin, "project", d.org, "secret")
	for _, ref := range []string{secret, "acme/secret", "acme/none", "nope"} {
		out, isErr = call(t, s, "get_project", map[string]any{"project": ref})
		eq(t, isErr, true, ref)
		eq(t, out, "not_found", ref)
	}
}

func TestMCPResultsAreBoundedAndResourcesRead(t *testing.T) {
	t.Parallel()
	d, token := mcpDocs(t, "KNOWLEDGE_MCP_MAX_RESULT_BYTES", "4096")
	d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"include": []string{"README.md", "big.md"}})
	d.push(map[string]string{"README.md": "# API", "big.md": strings.Repeat("word ", 4096)}, forgefake.Branch{Name: "main", SHA: sha(2)})
	d.app.collect()
	s := d.app.mcpSession(token)
	out, _ := call(t, s, "get_doc", map[string]any{"project": "acme/api", "path": "big.md"})
	eq(t, len(out) <= 4096, true, len(out))
	contains(t, out, "[truncated")

	templates, err := s.ListResourceTemplates(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, templates.ResourceTemplates[0].URITemplate, "svcr://projects/{project}/docs/{+path}")
	list, err := s.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, len(list.Resources), 2)
	read, err := s.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "svcr://projects/" + d.id + "/docs/README.md"})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, read.Contents[0].Text, "# API")
	eq(t, read.Contents[0].MIMEType, "text/markdown")
	if _, err := s.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "svcr://projects/" + d.id + "/docs/nope.md"}); err == nil {
		t.Fatal("missing resource read")
	}
}

func TestMCPEndpointRefusals(t *testing.T) {
	t.Parallel()
	d, token := mcpDocs(t)
	init := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	post := func(body string, headers ...string) reply {
		return request(t, d.app.addr, "POST", "/api/mcp", body, append([]string{"content-type", "application/json",
			"accept", "application/json, text/event-stream"}, headers...)...)
	}
	r := post(init)
	eq(t, r.status, 401)
	eq(t, code(t, r), any("auth.unauthenticated"))
	eq(t, r.header("WWW-Authenticate"), "Bearer")
	r = post(init, "cookie", d.admin)
	eq(t, r.status, 401, "the session does not count")
	r = post(init, "authorization", "Bearer svcp_nope")
	eq(t, r.status, 401)
	eq(t, code(t, r), any("auth.invalid_token"))
	eq(t, r.header("WWW-Authenticate"), "Bearer")
	r = post(init, "authorization", "Bearer "+d.app.pat(d.admin, "read", "write"))
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	r = post(init, "authorization", "Bearer "+token, "origin", "https://evil.example")
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.csrf_rejected"))
	r = post(init, "authorization", "Bearer "+token)
	eq(t, r.status, 200, r.text())
	var msg map[string]any
	if err := json.Unmarshal(r.body, &msg); err != nil {
		t.Fatal(r.text())
	}
	eq(t, msg["id"], any(float64(1)))
	r = post(`{"jsonrpc":"2.0","id":2,"method":"nope"}`, "authorization", "Bearer "+token)
	eq(t, r.status, 200)
	eq(t, r.text(), `{"jsonrpc":"2.0","id":2,"error":{"code":-32601,"message":"method not found: nope"}}`)
	r = post(`garbage`, "authorization", "Bearer "+token)
	contains(t, r.text(), `"code":-32700`)
	r = post(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_doc","arguments":{"project":1}}}`, "authorization", "Bearer "+token)
	contains(t, r.text(), `"isError":true`)
	r = request(t, d.app.addr, "GET", "/api/mcp", "", "authorization", "Bearer "+token)
	eq(t, r.status, 405)
	eq(t, r.header("Allow"), "POST")
	r = post(`{"pad":"`+strings.Repeat("x", 1<<20)+`"}`, "authorization", "Bearer "+token)
	eq(t, r.status, 413)
	eq(t, code(t, r), any("validation.invalid_body"))
	eq(t, d.app.bearer("GET", "/api/v1/catalog/nodes", token, nil).status, 403)
}
