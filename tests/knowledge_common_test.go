package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"svc-registry/internal/testsupport/forgefake"
)

type docsProject struct {
	app   *testApp
	fake  *forgefake.Fake
	repo  forgefake.Repo
	admin string
	org   string
	conn  string
	id    string
}

func startDocs(t *testing.T, files map[string]string, extra ...string) *docsProject {
	t.Helper()
	app := startForgeApp(t, extra...)
	root := app.admin()
	org := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	repo := forgefake.Repo{ID: 1, Path: "acme-inc/api", Files: files,
		Branches: []forgefake.Branch{{Name: "main", SHA: sha(1)}}}
	f.Put(repo)
	conn := idOf(app.connect(root, org, f, nil))
	if run := app.syncNow(root, org, conn); run["status"] != "succeeded" {
		t.Fatalf("sync: %v", run)
	}
	p := app.childBySlug(root, org, "api")
	return &docsProject{app: app, fake: f, repo: repo, admin: root, org: org, conn: conn, id: idOf(p)}
}

func sha(n int) string { return fmt.Sprintf("%040x", n) }

func (d *docsProject) push(files map[string]string, branches ...forgefake.Branch) {
	d.app.t.Helper()
	if files != nil {
		d.repo.Files = files
	}
	if branches != nil {
		d.repo.Branches = branches
	}
	if d.repo.Pushed.IsZero() {
		d.repo.Pushed = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	}
	d.repo.Pushed = d.repo.Pushed.Add(time.Hour)
	d.fake.Put(d.repo)
	if run := d.app.syncNow(d.admin, d.org, d.conn); run["status"] != "succeeded" {
		d.app.t.Fatalf("sync: %v", run)
	}
}

func (a *testApp) collect() int {
	a.t.Helper()
	ctx := context.Background()
	execSQL(a.t, a.db, "UPDATE knowledge_settings SET next_run_at = now()")
	claimed, err := a.services.Knowledge.ClaimKnowledge(ctx, 100, 120)
	if err != nil {
		a.t.Fatal(err)
	}
	for _, id := range claimed {
		if err := a.services.Knowledge.RunKnowledgeCollect(ctx, id); err != nil {
			a.t.Fatal(err)
		}
	}
	return len(claimed)
}

func (d *docsProject) snapshots(branch string) int64 {
	return scalar[int64](d.app.t, d.app.db, "SELECT count(*) FROM knowledge_snapshots WHERE project_id = $1 AND branch = $2", d.id, branch)
}

func (d *docsProject) files(branch string) string {
	return scalar[string](d.app.t, d.app.db, "SELECT COALESCE(string_agg(f.path || ':' || COALESCE(f.skip_reason, ''), ',' ORDER BY f.path), '') "+
		"FROM knowledge_files f WHERE f.snapshot_id = (SELECT id FROM knowledge_snapshots WHERE project_id = $1 AND branch = $2 "+
		"AND status <> 'failed' ORDER BY collected_at DESC, id DESC LIMIT 1)", d.id, branch)
}

func (d *docsProject) last(branch, column string) string {
	return scalar[string](d.app.t, d.app.db, "SELECT COALESCE("+column+"::text, '') FROM knowledge_snapshots WHERE project_id = $1 "+
		"AND branch = $2 ORDER BY collected_at DESC, id DESC LIMIT 1", d.id, branch)
}

func pruneBranches(t *testing.T, a *testApp) {
	t.Helper()
	if _, err := a.services.Catalog.PruneBranches(context.Background()); err != nil {
		t.Fatal(err)
	}
}
