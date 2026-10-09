package tests

import (
	"path/filepath"
	"testing"

	"svc-registry/internal/docsource"
	"svc-registry/internal/testsupport/forgefake"
)

func TestALocalDirectoryIsCollected(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	dir := filepath.Join(a.root, "api-docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# API docs")
	writeFile(t, filepath.Join(dir, "docs/a.md"), "alpha")
	r := a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_dir", "path": dir})
	eq(t, r.status, 200, r.text())
	a.send("PUT", knowledgePath(a.project)+"/settings", a.admin, obj{"include": []string{"**/*.md"}})
	a.collect()
	eq(t, a.snapshotsOf(a.project, "local"), int64(1))
	first := a.lastOf(a.project, "local", "commit_sha")
	eq(t, first[:7], "sha256:")
	k := a.get(knowledgePath(a.project), a.admin).json(t)
	eq(t, k["source"], any("local_dir"))
	eq(t, at(k, "branches", 0, "name"), any("local"))
	eq(t, a.get(knowledgePath(a.project)+"/file?path=docs%2Fa.md", a.admin).json(t)["content"], any("alpha"))

	a.collect()
	eq(t, a.snapshotsOf(a.project, "local"), int64(1))
	eq(t, a.lastOf(a.project, "local", "commit_sha"), first)
	writeFile(t, filepath.Join(dir, "docs/a.md"), "beta")
	a.collect()
	eq(t, a.snapshotsOf(a.project, "local"), int64(2))
	eq(t, a.get(knowledgePath(a.project)+"/file?path=docs%2Fa.md", a.admin).json(t)["content"], any("beta"))
}

func TestALocalGitRepositoryIsCollectedByBranch(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	g := newGitRepo(t, filepath.Join(a.root, "repo"))
	main := g.commit(map[string]string{"README.md": "main one"})
	g.branch("release/1")
	rel := g.commit(map[string]string{"README.md": "release one"})
	g.branch("dev")
	g.commit(map[string]string{"README.md": "dev"})
	eq(t, a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_git", "path": g.dir}).status, 200)
	a.send("PUT", knowledgePath(a.project)+"/settings", a.admin, obj{"include": nil, "branches": []string{"release/*"}})
	a.collect()
	eq(t, a.lastOf(a.project, "release/1", "commit_sha"), rel)
	eq(t, a.snapshotsOf(a.project, "dev"), int64(1))
	eq(t, a.snapshotsOf(a.project, "master"), int64(0), "not the default, not a pattern")
	_ = main
	eq(t, a.send("PATCH", nodePath(a.project), a.admin, obj{"default_branch": "master"}).status, 200)
	a.collect()
	eq(t, a.lastOf(a.project, "master", "commit_sha"), main)
	g.branch("release/2")
	rel2 := g.commit(map[string]string{"README.md": "release two"})
	writeFile(t, filepath.Join(g.dir, "README.md"), "uncommitted")
	a.collect()
	eq(t, a.lastOf(a.project, "release/2", "commit_sha"), rel2)
	r := a.get(knowledgePath(a.project)+"/file?path=README.md&branch=release%2F2", a.admin)
	eq(t, r.json(t)["content"], any("release two"))
	eq(t, a.get(knowledgePath(a.project), a.admin).json(t)["source"], any("local_git"))
}

func TestARemoteRepositoryIsCollectedThroughTheForge(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	f := forgefake.Start(t, "github", forgeToken, "other")
	f.Put(forgefake.Repo{ID: 1, Path: "other/api", DefaultBranch: "main", Files: map[string]string{"README.md": "# Remote"},
		Branches: []forgefake.Branch{{Name: "main", SHA: sha(7)}}})
	r := a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "remote", "forge": "github",
		"url": "https://github.example/other/api", "api_url": f.APIURL(), "credentials": obj{"token": forgeToken}})
	eq(t, r.status, 200, r.text())
	a.collect()
	eq(t, a.lastOf(a.project, "main", "commit_sha"), sha(7))
	eq(t, a.get(knowledgePath(a.project)+"/file?path=README.md", a.admin).json(t)["content"], any("# Remote"))

	g := forgefake.Start(t, "gitlab", "unused", "pub")
	g.Anonymous = true
	g.Put(forgefake.Repo{ID: 5, Path: "pub/site", DefaultBranch: "main", Files: map[string]string{"README.md": "# Public"},
		Branches: []forgefake.Branch{{Name: "main", SHA: sha(8)}}})
	other := a.nodeID(a.admin, "project", a.org, "site")
	r = a.send("PUT", sourcePath(other), a.admin, obj{"kind": "remote", "forge": "gitlab",
		"url": "https://gitlab.example/pub/site", "api_url": g.APIURL() + "/api/v4", "credentials": nil})
	eq(t, r.status, 200, r.text())
	a.collect()
	eq(t, a.lastOf(other, "main", "commit_sha"), sha(8))
	r = a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "remote", "forge": "github",
		"url": "https://github.example/other/api", "api_url": f.APIURL(), "credentials": obj{"token": "wrong"}})
	eq(t, r.status, 200)
	a.collect()
	eq(t, a.lastOf(a.project, "main", "error_code"), "forge.unauthorized")
}

func TestASourceWinsOverTheForgeAndFailuresAreRecorded(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "from the forge"}, "KNOWLEDGE_LOCAL_ROOTS", t.TempDir())
	roots := d.app.state.Config.Knowledge.LocalRoots
	dir := filepath.Join(roots[0], "local")
	writeFile(t, filepath.Join(dir, "README.md"), "from the disk")
	eq(t, d.app.send("PUT", sourcePath(d.id), d.admin, obj{"kind": "local_dir", "path": dir}).status, 200)
	d.app.collect()
	eq(t, d.snapshots("local"), int64(1))
	eq(t, d.app.get(knowledgePath(d.id)+"/file?path=README.md", d.admin).json(t)["content"], any("from the disk"))

	if err := removeAll(dir); err != nil {
		t.Fatal(err)
	}
	d.app.collect()
	eq(t, d.last("local", "error_code"), "source.not_found")
	eq(t, d.app.get(knowledgePath(d.id)+"/file?path=README.md", d.admin).json(t)["content"], any("from the disk"))
	writeFile(t, filepath.Join(dir, "README.md"), "back")
	d.app.state.DocReaders.(*docsource.Factory).Roots = nil
	d.app.collect()
	eq(t, d.last("local", "error_code"), "source.not_allowed")
	eq(t, d.app.call("DELETE", sourcePath(d.id), d.admin).status, 204)
	d.app.collect()
	eq(t, d.app.get(knowledgePath(d.id), d.admin).json(t)["source"], any("forge"))
	eq(t, d.app.get(knowledgePath(d.id)+"/file?path=README.md", d.admin).json(t)["content"], any("from the forge"))
}
