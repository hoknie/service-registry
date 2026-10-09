package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type sourcesApp struct {
	*testApp
	root    string
	admin   string
	org     string
	project string
}

func startSources(t *testing.T, extra ...string) *sourcesApp {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := startForgeApp(t, append([]string{"KNOWLEDGE_LOCAL_ROOTS", root}, extra...)...)
	admin := app.admin()
	org := app.nodeID(admin, "organization", "", "acme")
	return &sourcesApp{testApp: app, root: root, admin: admin, org: org, project: app.nodeID(admin, "project", org, "api")}
}

func sourcePath(id string) string { return nodePath(id) + "/knowledge/source" }

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type gitRepo struct {
	t    *testing.T
	dir  string
	repo *git.Repository
	wt   *git.Worktree
}

func newGitRepo(t *testing.T, dir string) *gitRepo {
	t.Helper()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := repo.Worktree()
	return &gitRepo{t: t, dir: dir, repo: repo, wt: wt}
}

func (g *gitRepo) commit(files map[string]string) string {
	g.t.Helper()
	for p, c := range files {
		writeFile(g.t, filepath.Join(g.dir, p), c)
		if _, err := g.wt.Add(p); err != nil {
			g.t.Fatal(err)
		}
	}
	h, err := g.wt.Commit("change", &git.CommitOptions{Author: &object.Signature{Name: "t", Email: "t@example.com", When: time.Now()}})
	if err != nil {
		g.t.Fatal(err)
	}
	return h.String()
}

func (g *gitRepo) branch(name string) {
	g.t.Helper()
	if err := g.wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(name), Create: true}); err != nil {
		g.t.Fatal(err)
	}
}

func (a *sourcesApp) snapshotsOf(project, branch string) int64 {
	return scalar[int64](a.t, a.db, "SELECT count(*) FROM knowledge_snapshots WHERE project_id = $1 AND branch = $2 AND status <> 'failed'", project, branch)
}

func (a *sourcesApp) lastOf(project, branch, column string) string {
	return scalar[string](a.t, a.db, "SELECT COALESCE("+column+"::text, '') FROM knowledge_snapshots WHERE project_id = $1 "+
		"AND branch = $2 ORDER BY collected_at DESC, id DESC LIMIT 1", project, branch)
}

func removeAll(path string) error { return os.RemoveAll(path) }
