package tests

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	qd "github.com/qdrant/go-client/qdrant"

	"svc-registry/internal/feature/knowledge/search/qdrant"
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

func qdrantPoints(t *testing.T, addr, collection string) uint64 {
	t.Helper()
	host, port, _ := strings.Cut(addr, ":")
	p, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	c, err := qd.NewClient(&qd.Config{Host: host, Port: p, SkipCompatibilityCheck: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	n, err := c.Count(context.Background(), &qd.CountPoints{CollectionName: collection})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSearchQdrantCollectionOfOtherDimensions(t *testing.T) {
	t.Parallel()
	addr := os.Getenv("TEST_QDRANT_URL")
	if addr == "" {
		t.Skip("TEST_QDRANT_URL is not set; `just test` sets it")
	}
	name := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := qdrant.New(addr, "", name, embedDims*2).Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := startSearch(t, "qdrant", "QDRANT_COLLECTION", name)
	s.docs(map[string]string{"README.md": "# Registry\nThe service registry keeps projects."})
	s.collect()
	s.index()

	failed := scalar[string](t, s.db, "SELECT error_code FROM knowledge_scans WHERE project_id = $1 AND kind = 'index' ORDER BY last_started_at DESC LIMIT 1", s.project)
	eq(t, failed, "search.engine_dimensions")
	detail := scalar[string](t, s.db, "SELECT error_detail FROM knowledge_scans WHERE project_id = $1 AND kind = 'index' ORDER BY last_started_at DESC LIMIT 1", s.project)
	for _, part := range []string{name, strconv.Itoa(embedDims * 2), strconv.Itoa(embedDims), "QDRANT_COLLECTION"} {
		contains(t, detail, part)
	}
	r := s.get(searchPath("registry", "mode", "semantic"), s.admin)
	eq(t, r.status, 503, r.text())
	eq(t, code(t, r), any("search.unavailable"))
	contains(t, projectsIn(t, s.testApp, s.admin, "registry", "mode", "text"), "acme/api:README.md")
	eq(t, qdrantPoints(t, addr, name), uint64(0), "the collection is left alone")

	fresh := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	s.restart("QDRANT_COLLECTION", fresh)
	s.index()
	status := scalar[string](t, s.db, "SELECT status FROM knowledge_scans WHERE project_id = $1 AND kind = 'index' ORDER BY last_started_at DESC LIMIT 1", s.project)
	eq(t, status, "ok")
	contains(t, projectsIn(t, s.testApp, s.admin, "registry", "mode", "semantic"), "acme/api:README.md")
}
