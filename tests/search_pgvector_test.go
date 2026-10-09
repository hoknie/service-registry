package tests

import (
	"slices"
	"strings"
	"testing"
)

func searchPaths(t *testing.T, a *testApp, cookie, q string, extra ...string) string {
	t.Helper()
	r := a.get(searchPath(q, extra...), cookie)
	eq(t, r.status, 200, r.text())
	var out []string
	for _, it := range list(t, r.json(t), "items") {
		out = append(out, it.(obj)["path"].(string))
	}
	return strings.Join(out, ",")
}

func seedSearch(t *testing.T, engine string, extra ...string) *searchApp {
	t.Helper()
	s := startSearch(t, engine, extra...)
	s.docs(map[string]string{
		"README.md":        "# Registry\nThe service registry keeps projects.",
		"docs/rollback.md": "# Rollback\nHow to revert a release safely: run the previous version.",
		"docs/limits.md":   "# Limits\nRequest quotas per token.",
	})
	s.collect()
	s.index()
	return s
}

func TestSearchPgvectorModes(t *testing.T) {
	t.Parallel()
	s := seedSearch(t, "pgvector")
	r := s.get("/api/v1/knowledge/search/modes", s.admin)
	eq(t, r.status, 200, r.text())
	contains(t, r.text(), `"engine":"pgvector","modes":["text","semantic","hybrid"],"default":"hybrid"`)
	eq(t, at(r.json(t), "index", "pending"), any(float64(0)))

	got := searchPaths(t, s.testApp, s.admin, "как откатить релиз", "mode", "semantic")
	eq(t, strings.HasPrefix(got, "docs/rollback.md"), true, got)
	eq(t, searchPaths(t, s.testApp, s.admin, "как откатить релиз", "mode", "text"), "", "no such words in the text")
	hybrid := strings.Split(searchPaths(t, s.testApp, s.admin, "quotas", "mode", "hybrid"), ",")
	eq(t, hybrid[0], "docs/limits.md", hybrid)
	eq(t, slices.Contains(hybrid, "docs/limits.md"), true)
	eq(t, strings.HasPrefix(searchPaths(t, s.testApp, s.admin, "как откатить релиз"), "docs/rollback.md"), true, "hybrid by default")

	bob := bobSession(s.testApp)
	eq(t, searchPaths(t, s.testApp, bob, "как откатить релиз", "mode", "semantic"), "", "an invisible project")
	s.grant(s.admin, s.project, "user", "bob@example.com", "viewer")
	eq(t, strings.HasPrefix(searchPaths(t, s.testApp, bob, "как откатить релиз", "mode", "semantic"), "docs/rollback.md"), true)

	execSQL(t, s.db, "DELETE FROM knowledge_snapshots")
	eq(t, searchPaths(t, s.testApp, s.admin, "как откатить релиз", "mode", "semantic"), "", "gone with its snapshots")

	r = s.get(searchPath("x", "mode", "vector"), s.admin)
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.search_mode_unavailable"))
}

func TestSearchSemanticUnavailableWithoutEmbeddings(t *testing.T) {
	t.Parallel()
	s := seedSearch(t, "pgvector")
	s.embed.Break(true)
	r := s.get(searchPath("как откатить релиз", "mode", "semantic"), s.admin)
	eq(t, r.status, 503, r.text())
	eq(t, code(t, r), any("search.unavailable"))
	eq(t, s.get(searchPath("registry", "mode", "text"), s.admin).status, 200, "text keeps working")
}

func TestSearchModesWithTokensAndMCP(t *testing.T) {
	t.Parallel()
	s := seedSearch(t, "pgvector")
	read := s.pat(s.admin, "read")
	eq(t, s.bearer("GET", "/api/v1/knowledge/search/modes", read, nil).status, 200)
	r := s.bearer("GET", "/api/v1/knowledge/search/modes", s.pat(s.admin, "write"), nil)
	eq(t, r.status, 403, r.text())
	eq(t, code(t, r), any("auth.insufficient_scope"))
	eq(t, s.get("/api/v1/knowledge/search/modes", "").status, 401)

	m := s.mcpSession(s.pat(s.admin, "mcp", "admin"))
	out, isErr := call(t, m, "search_docs", map[string]any{"query": "как откатить релиз", "mode": "semantic"})
	eq(t, isErr, false, out)
	contains(t, out, `"path": "docs/rollback.md"`)
	out, isErr = call(t, m, "search_docs", map[string]any{"query": "x", "mode": "vector"})
	eq(t, isErr, true, out)
	contains(t, out, "validation.search_mode_unavailable")
}

func TestSearchModesOfPostgres(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	r := a.get("/api/v1/knowledge/search/modes", admin)
	eq(t, r.status, 200, r.text())
	eq(t, r.text(), `{"engine":"postgres","modes":["text"],"default":"text","index":null}`)
	r = a.get(searchPath("x", "mode", "semantic"), admin)
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.search_mode_unavailable"))
	eq(t, a.get(searchPath("x", "mode", "text"), admin).status, 200)
}
