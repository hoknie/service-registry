package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

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
	eq(t, a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_git", "path": g.dir, "working_tree": false}).status, 200)
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

func TestLocalGitCollectsTheWorkingTreeOfTheCheckedOutBranch(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	dir := filepath.Join(a.root, "repo")
	g := newGitRepo(t, dir)
	head := g.commit(map[string]string{"README.md": "# Repo", ".gitignore": "build/\n"})
	writeFile(t, filepath.Join(dir, "openspec", "specs", "a", "spec.md"), "## Purpose\nNot committed yet.")
	writeFile(t, filepath.Join(dir, "build", "out.md"), "ignored")
	a.useSource(a.project, "local_git", dir, "**/*.md")
	a.collect()
	branch := scalar[string](t, a.db, "SELECT branch FROM knowledge_snapshots WHERE project_id = $1 ORDER BY collected_at DESC LIMIT 1", a.project)
	commit := a.lastOf(a.project, branch, "commit_sha")
	eq(t, strings.HasPrefix(commit, head+"+worktree:sha256:"), true, commit)
	files := scalar[string](t, a.db, "SELECT string_agg(f.path, ',' ORDER BY f.path COLLATE \"C\") FROM knowledge_files f JOIN knowledge_snapshots s ON s.id = f.snapshot_id "+
		"WHERE s.project_id = $1 AND s.commit_sha = $2", a.project, commit)
	eq(t, files, "README.md,openspec/specs/a/spec.md")
	s := a.scans(a.project, "collect")
	eq(t, s[0].Status, "ok")
	contains(t, s[0].Branches, `"working_tree": true`)

	before := a.snapshotsOf(a.project, branch)
	a.collect()
	eq(t, a.snapshotsOf(a.project, branch), before, "no edits, no new snapshot")
	writeFile(t, filepath.Join(dir, "openspec", "specs", "a", "spec.md"), "## Purpose\nEdited.")
	a.collect()
	eq(t, a.snapshotsOf(a.project, branch), before+1, "an edit makes a snapshot")

	eq(t, a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_git", "path": dir, "working_tree": false}).status, 200)
	a.collect()
	eq(t, a.lastOf(a.project, branch, "commit_sha"), head, "commits only")
}

func TestLocalGitCollectsIgnoredDocumentationOnRequest(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	dir := filepath.Join(a.root, "repo")
	g := newGitRepo(t, dir)
	head := g.commit(map[string]string{"README.md": "# Repo", ".gitignore": "*.md\n"})
	writeFile(t, filepath.Join(dir, "openspec", "specs", "a", "spec.md"), "## Purpose\nKept out of git.")
	a.useSource(a.project, "local_git", dir, "openspec/**/*.md", "*.md")
	a.collect()
	branch := scalar[string](t, a.db, "SELECT branch FROM knowledge_snapshots WHERE project_id = $1 ORDER BY collected_at DESC LIMIT 1", a.project)
	files := func(commit string) string {
		return scalar[string](t, a.db, "SELECT string_agg(f.path, ',' ORDER BY f.path COLLATE \"C\") FROM knowledge_files f "+
			"JOIN knowledge_snapshots s ON s.id = f.snapshot_id WHERE s.project_id = $1 AND s.commit_sha = $2", a.project, commit)
	}
	eq(t, a.lastOf(a.project, branch, "commit_sha"), head, "ignored files stay out")
	eq(t, files(head), "README.md")

	r := a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_git", "path": dir, "include_ignored": true})
	eq(t, r.status, 200, r.text())
	a.collect()
	commit := a.lastOf(a.project, branch, "commit_sha")
	eq(t, strings.HasPrefix(commit, head+"+worktree:sha256:"), true, commit)
	eq(t, files(commit), "README.md,openspec/specs/a/spec.md")
}

func TestSourceBranchesAreRecordedInTheProjectBranches(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	dir := filepath.Join(a.root, "repo")
	g := newGitRepo(t, dir)
	main := g.commit(map[string]string{"README.md": "# Repo"})
	g.branch("release/1")
	rel := g.commit(map[string]string{"README.md": "release"})
	if err := g.wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("master")}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "notes.md"), "uncommitted")
	a.useSource(a.project, "local_git", dir, "*.md")
	a.collect()
	branches := func() map[string]obj {
		out := map[string]obj{}
		for _, b := range a.get(nodePath(a.project)+"/branches?state=all", a.admin).json(t)["items"].([]any) {
			out[b.(obj)["name"].(string)] = b.(obj)
		}
		return out
	}
	b := branches()
	eq(t, b["master"]["head_sha"], any(main), "the commit, not the working copy")
	eq(t, b["master"]["is_default"], any(true))
	eq(t, jsonText(b["master"]["sources"]), `["repository"]`)
	eq(t, b["release/1"]["head_sha"], any(rel))
	eq(t, b["release/1"]["gone_at"], nil)

	if err := g.repo.Storer.RemoveReference(plumbing.NewBranchReferenceName("release/1")); err != nil {
		t.Fatal(err)
	}
	a.collect()
	b = branches()
	eq(t, b["release/1"]["gone_at"] != nil, true, "vanished from the source")
	eq(t, b["master"]["gone_at"], nil)

	other := a.nodeID(a.admin, "project", a.org, "plain")
	docs := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(docs, "README.md"), "# Docs")
	a.useSource(other, "local_dir", docs, "*.md")
	a.collect()
	eq(t, a.get(nodePath(other)+"/branches?state=all", a.admin).json(t)["total"], any(float64(0)), "a local directory has no branches")
}
