package tests

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"svc-registry/pkg/testing/hostingfake"
)

func (d *docsProject) tags(tags ...hostingfake.Tag) {
	d.app.t.Helper()
	d.repo.Tags = tags
	d.fake.SetTags(d.repo.ID, tags)
}

func (d *docsProject) refs(refs ...obj) {
	d.app.t.Helper()
	if refs == nil {
		refs = []obj{}
	}
	r := d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"include": []string{"README.md"}, "refs": refs})
	eq(d.app.t, r.status, 200)
}

func tagNames(t *testing.T, k obj) string {
	t.Helper()
	var names []string
	for _, v := range k["tags"].([]any) {
		tag := v.(obj)
		name := tag["name"].(string)
		if tag["release"].(bool) {
			name += "*"
		}
		names = append(names, name)
	}
	return strings.Join(names, ",")
}

func tagged(name string, n int) hostingfake.Tag { return hostingfake.Tag{Name: name, SHA: sha(n)} }

func TestKnowledgeTagsLimitEvictionAndRepeat(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "# Tags"}, "KNOWLEDGE_MAX_TAGS", "2")
	d.refs(obj{"pattern": "v*", "kind": "tag"})
	d.tags(tagged("v1.0.0", 10), tagged("v1.1.0", 11), tagged("v1.2.0", 12), tagged("other", 13))
	d.app.collect()
	k := d.app.get(knowledgePath(d.id), d.admin).json(t)
	eq(t, tagNames(t, k), "v1.2.0,v1.1.0")
	eq(t, k["release"], any(nil))
	eq(t, d.app.get(knowledgePath(d.id)+"/files?tag=v1.0.0", d.admin).status, 404)
	f := d.app.get(knowledgePath(d.id)+"/files?tag=v1.2.0", d.admin)
	eq(t, f.status, 200)
	eq(t, at(f.json(t), "snapshot", "ref_kind"), any("tag"))
	eq(t, at(f.json(t), "snapshot", "commit"), any(sha(12)))
	eq(t, at(d.app.get(knowledgePath(d.id)+"/files", d.admin).json(t), "snapshot", "ref_kind"), any("branch"))

	d.tags(tagged("v1.0.0", 10), tagged("v1.1.0", 11), tagged("v1.2.0", 12), tagged("v1.3.0", 14))
	d.app.collect()
	eq(t, tagNames(t, d.app.get(knowledgePath(d.id), d.admin).json(t)), "v1.3.0,v1.2.0")
	eq(t, d.app.get(knowledgePath(d.id)+"/files?tag=v1.1.0", d.admin).status, 404)
	eq(t, scalar[int64](t, d.app.db, "SELECT count(*) FROM knowledge_snapshots WHERE ref_kind = 'tag' AND branch = 'v1.1.0'"), int64(0))

	d.app.collect()
	eq(t, scalar[int64](t, d.app.db, "SELECT count(*) FROM knowledge_snapshots WHERE ref_kind = 'tag' AND branch = 'v1.3.0'"), int64(1))
	eq(t, scalar[string](t, d.app.db, `SELECT b ->> 'result' FROM knowledge_scans k, jsonb_array_elements(k.branches) b
		WHERE k.project_id = $1 AND b ->> 'kind' = 'tag' AND b ->> 'name' = 'v1.3.0' ORDER BY k.last_started_at DESC LIMIT 1`, d.id), "unchanged")

	d.tags(tagged("v1.2.0", 12), tagged("v1.3.0", 15))
	d.app.collect()
	eq(t, at(d.app.get(knowledgePath(d.id)+"/files?tag=v1.3.0", d.admin).json(t), "snapshot", "commit"), any(sha(15)))

	d.refs()
	d.app.collect()
	eq(t, tagNames(t, d.app.get(knowledgePath(d.id), d.admin).json(t)), "")
	eq(t, scalar[int64](t, d.app.db, "SELECT count(*) FROM knowledge_snapshots WHERE ref_kind = 'tag'"), int64(0))
}

func TestKnowledgeCurrentReleaseMoves(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "# Release"})
	d.refs(obj{"pattern": "v*", "kind": "tag"}, obj{"pattern": "v*-release", "kind": "tag", "release": true})
	d.tags(tagged("v0.1.0-release", 20))
	d.app.collect()
	k := d.app.get(knowledgePath(d.id), d.admin).json(t)
	eq(t, k["release"], any("v0.1.0-release"))

	d.tags(tagged("v0.1.0-release", 20), tagged("v0.2.0-release", 21))
	d.app.collect()
	k = d.app.get(knowledgePath(d.id), d.admin).json(t)
	eq(t, k["release"], any("v0.2.0-release"))
	eq(t, tagNames(t, k), "v0.2.0-release*,v0.1.0-release")

	d.tags(tagged("v0.1.0-release", 20), tagged("v0.2.0-release", 21), tagged("v0.3.0-rc1", 22))
	d.app.collect()
	k = d.app.get(knowledgePath(d.id), d.admin).json(t)
	eq(t, tagNames(t, k), "v0.3.0-rc1,v0.2.0-release*,v0.1.0-release")
	r := d.app.get(knowledgePath(d.id)+"/file?path=README.md&release=current", d.admin)
	eq(t, r.status, 200)
	eq(t, at(r.json(t), "snapshot", "branch"), any("v0.2.0-release"))
	eq(t, at(r.json(t), "snapshot", "ref_kind"), any("tag"))

	space := d.app.get("/api/v1/docs/space?path=acme/api", d.admin).json(t)
	eq(t, at(space, "docs", "release"), any("v0.2.0-release"))
	eq(t, at(space, "docs", "tags", 1, "name"), any("v0.2.0-release"))
	eq(t, at(space, "docs", "tags", 1, "release"), any(true))

	d.tags(tagged("v0.1.0-release", 20), tagged("v0.3.0-rc1", 22))
	d.app.collect()
	eq(t, d.app.get(knowledgePath(d.id), d.admin).json(t)["release"], any("v0.1.0-release"))
	eq(t, scalar[int64](t, d.app.db, "SELECT count(*) FROM knowledge_tags WHERE release"), int64(1))
}

func TestKnowledgeReleaseOutsideTheLimit(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "# Release"}, "KNOWLEDGE_MAX_TAGS", "1")
	d.refs(obj{"pattern": "v*", "kind": "tag"}, obj{"pattern": "v*-release", "kind": "tag", "release": true})
	d.tags(tagged("v0.2.0-release", 30), tagged("v0.3.0-rc1", 31))
	d.app.collect()
	eq(t, tagNames(t, d.app.get(knowledgePath(d.id), d.admin).json(t)), "v0.3.0-rc1,v0.2.0-release*")
}

func TestKnowledgeTagSelectorsAndSettings(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "# Deploy notes"})
	d.refs(obj{"pattern": "v*", "kind": "tag"})
	d.tags(tagged("v1.0.0", 40))
	d.app.collect()
	base := knowledgePath(d.id)
	for _, q := range []string{"branch=main&tag=v1.0.0", "release=latest", "tag=", "tag=" + strings.Repeat("a", 256)} {
		r := d.app.get(base+"/files?"+q, d.admin)
		eq(t, r.status, 400)
		eq(t, code(t, r), any("validation.invalid_ref"))
	}
	eq(t, d.app.get(base+"/files?release=current", d.admin).status, 404)
	eq(t, d.app.get(base+"/files?tag=v9", d.admin).status, 404)
	eq(t, d.app.get(base+"/tree?tag=v1.0.0", d.admin).status, 200)

	r := d.app.send("PUT", base+"/settings", d.admin, obj{"branches": []string{"release/*"}})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.knowledge_branches_replaced"))
	for _, refs := range [][]obj{
		{{"pattern": "main", "kind": "branch", "release": true}},
		{{"pattern": "v*", "kind": "commit"}},
		{{"pattern": "v*", "kind": "tag"}, {"pattern": "v*", "kind": "tag"}},
	} {
		r := d.app.send("PUT", base+"/settings", d.admin, obj{"refs": refs})
		eq(t, r.status, 400)
		eq(t, code(t, r), any("validation.invalid_knowledge_settings"))
	}

	s := d.app.get("/api/v1/knowledge/search?q=deploy&tag=v1.0.0", d.admin)
	eq(t, s.status, 200)
	eq(t, at(s.json(t), "items", 0, "branch"), any("v1.0.0"))
	eq(t, at(s.json(t), "items", 0, "ref_kind"), any("tag"))
	eq(t, at(d.app.get("/api/v1/knowledge/search?q=deploy", d.admin).json(t), "items", 0, "ref_kind"), any("branch"))
	eq(t, len(d.app.get("/api/v1/knowledge/search?q=deploy&release=current", d.admin).json(t)["items"].([]any)), 0)
	eq(t, code(t, d.app.get("/api/v1/knowledge/search?q=deploy&branch=main&release=current", d.admin)), any("validation.invalid_ref"))
}

func TestKnowledgeSearchAndMCPReadTheRelease(t *testing.T) {
	t.Parallel()
	d, token := mcpDocs(t)
	d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"include": []string{"README.md", "openspec/**"},
		"refs": []obj{{"pattern": "v*-release", "kind": "tag", "release": true}}})
	d.tags(tagged("v0.2.0-release", 50))
	d.app.collect()
	s := d.app.get("/api/v1/knowledge/search?q=idempotency&release=current", d.admin).json(t)
	items := s["items"].([]any)
	eq(t, len(items), 1)
	eq(t, at(items[0], "branch"), any("v0.2.0-release"))
	eq(t, at(items[0], "ref_kind"), any("tag"))

	m := d.app.mcpSession(token)
	out, failed := call(t, m, "get_doc", map[string]any{"project": "acme/api", "path": "README.md", "release": "current"})
	eq(t, failed, false)
	contains(t, out, "branch: v0.2.0-release")
	out, failed = call(t, m, "get_doc", map[string]any{"project": "acme/api", "path": "README.md", "branch": "main", "tag": "v0.2.0-release"})
	eq(t, failed, true)
	contains(t, out, "validation.invalid_ref")
	out, _ = call(t, m, "search_docs", map[string]any{"query": "idempotency", "tag": "v0.2.0-release"})
	contains(t, out, `"ref_kind": "tag"`)
	out, _ = call(t, m, "list_specs", map[string]any{"project": "acme/api", "release": "current"})
	contains(t, out, "ingest/events")
}

func TestLocalGitTagsAndSourceErrors(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	dir := filepath.Join(a.root, "repo")
	g := newGitRepo(t, dir)
	first := g.commit(map[string]string{"README.md": "one", ".svc-registry.yaml": "dependencies:\n  - external:old\n"})
	if _, err := g.repo.CreateTag("v0.1.0", plumbing.NewHash(first), &git.CreateTagOptions{Message: "release",
		Tagger: &object.Signature{Name: "t", Email: "t@example.com", When: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	g.commit(map[string]string{"README.md": "two", ".svc-registry.yaml": "dependencies: []\n"})
	eq(t, a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_git", "path": dir, "working_tree": false}).status, 200)
	a.send("PUT", knowledgePath(a.project)+"/settings", a.admin, obj{"include": []string{"README.md"},
		"refs": []obj{{"pattern": "v*", "kind": "tag", "release": true}}})
	a.collect()
	k := a.get(knowledgePath(a.project), a.admin).json(t)
	eq(t, k["release"], any("v0.1.0"))
	eq(t, at(k, "tags", 0, "commit"), any(first))
	f := a.get(knowledgePath(a.project)+"/file?path=README.md&tag=v0.1.0", a.admin).json(t)
	eq(t, f["content"], any("one"))
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM service_dependencies WHERE project_id = $1", a.project), int64(0))

	if err := removeAll(dir); err != nil {
		t.Fatal(err)
	}
	a.collect()
	eq(t, a.get(knowledgePath(a.project), a.admin).json(t)["release"], any("v0.1.0"))
	eq(t, a.get(knowledgePath(a.project)+"/files?tag=v0.1.0", a.admin).status, 200)
}
