package tests

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func projectsIn(t *testing.T, a *testApp, cookie, q string, extra ...string) string {
	t.Helper()
	r := a.get(searchPath(q, extra...), cookie)
	eq(t, r.status, 200, r.text())
	var out []string
	for _, it := range list(t, r.json(t), "items") {
		out = append(out, at(it, "project", "path").(string)+":"+it.(obj)["path"].(string))
	}
	return strings.Join(out, ",")
}

func externalEngine(t *testing.T, engine string) {
	s := startSearch(t, engine)
	rollback := "# Rollback\nHow to revert a release safely: run the previous version."
	s.docs(map[string]string{"README.md": "# Registry\nThe service registry keeps projects.", "docs/rollback.md": rollback})
	secret := s.nodeID(s.admin, "project", s.org, "secret")
	s.docsOf(secret, filepath.Join(s.root, "secret"), map[string]string{"docs/rollback.md": rollback + "\nSecret notes."})
	old := s.nodeID(s.admin, "project", s.org, "old")
	s.docsOf(old, filepath.Join(s.root, "old"), map[string]string{"docs/rollback.md": rollback + "\nOld notes."})
	s.collect()
	s.index()

	r := s.get("/api/v1/knowledge/search/modes", s.admin)
	eq(t, r.status, 200, r.text())
	contains(t, r.text(), `"engine":"`+engine+`","modes":["text","semantic","hybrid"],"default":"hybrid"`)

	got := projectsIn(t, s.testApp, s.admin, "как откатить релиз", "mode", "semantic")
	contains(t, got, "acme/api:docs/rollback.md")
	contains(t, got, "acme/secret:docs/rollback.md")
	contains(t, projectsIn(t, s.testApp, s.admin, "как откатить релиз", "mode", "hybrid"), "acme/api:docs/rollback.md")
	eq(t, strings.Count(got, "acme/api:docs/rollback.md"), 1, "one item per file", got)
	eq(t, projectsIn(t, s.testApp, s.admin, "как откатить релиз", "mode", "semantic", "path", "README"), "acme/api:README.md")

	bob := bobSession(s.testApp)
	s.grant(s.admin, s.project, "user", "bob@example.com", "viewer")
	got = projectsIn(t, s.testApp, bob, "как откатить релиз", "mode", "semantic")
	contains(t, got, "acme/api:docs/rollback.md")
	eq(t, strings.Contains(got, "acme/secret"), false, got)

	execSQL(t, s.db, "DELETE FROM knowledge_snapshots WHERE project_id = '"+secret+"'")
	s.index()
	eq(t, strings.Contains(projectsIn(t, s.testApp, s.admin, "как откатить релиз", "mode", "semantic"), "acme/secret"), false, "branch gone")

	execSQL(t, s.db, "DELETE FROM nodes WHERE id = '"+old+"'")
	if err := s.services.Knowledge.RetainExternalIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	execSQL(t, s.db, "INSERT INTO nodes (id, kind, parent_id, name, slug) SELECT '"+old+"', 'project', '"+s.org+"', 'old', 'old'")
	s.grant(s.admin, old, "user", "bob@example.com", "viewer")
	eq(t, strings.Contains(projectsIn(t, s.testApp, s.admin, "как откатить релиз", "mode", "semantic"), "acme/old"), false, "project gone")
}

func TestSearchQdrant(t *testing.T) {
	t.Parallel()
	externalEngine(t, "qdrant")
}

func TestSearchMeilisearch(t *testing.T) {
	t.Parallel()
	externalEngine(t, "meilisearch")
}

func TestSearchMeilisearchTextOnly(t *testing.T) {
	t.Parallel()
	s := startTextSearch(t, "meilisearch")
	s.docs(map[string]string{"README.md": "# Registry\nEvery repository is a project."})
	s.collect()
	s.index()
	r := s.get("/api/v1/knowledge/search/modes", s.admin)
	eq(t, r.status, 200, r.text())
	contains(t, r.text(), `"engine":"meilisearch","modes":["text"],"default":"text","index":null`)
	r = s.get(searchPath("repositry", "mode", "text"), s.admin)
	eq(t, r.status, 200, r.text())
	contains(t, r.text(), `"path":"README.md"`)
	contains(t, r.text(), `"match":true`)
	eq(t, s.get(searchPath("repositry", "mode", "semantic"), s.admin).status, 400)

	s.restart("MEILISEARCH_INDEX", "t_"+strings.ReplaceAll(uuid.NewString(), "-", ""))
	s.index()
	contains(t, s.get(searchPath("repositry", "mode", "text"), s.admin).text(), `"path":"README.md"`)
}

func TestSearchExternalEngineDown(t *testing.T) {
	t.Parallel()
	s := startSearch(t, "qdrant", "QDRANT_URL", "127.0.0.1:1")
	s.docs(map[string]string{"README.md": "# Registry\nThe service registry keeps projects."})
	s.collect()
	s.index()
	r := s.get(searchPath("registry", "mode", "semantic"), s.admin)
	eq(t, r.status, 503, r.text())
	eq(t, code(t, r), any("search.unavailable"))
	contains(t, projectsIn(t, s.testApp, s.admin, "registry", "mode", "text"), "acme/api:README.md")
}
