package tests

import (
	"strings"
	"testing"
	"time"

	"svc-registry/internal/testsupport/forgefake"
)

func TestTheReadmeIsCollectedByDefault(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "# API\nidempotency key", "docs/a.md": "a", "openspec/specs/x/spec.md": "## Purpose\nX"})
	eq(t, d.app.collect(), 1)
	eq(t, d.snapshots("main"), int64(1))
	eq(t, d.files("main"), "README.md:")
	eq(t, d.last("main", "commit_sha"), sha(1))
	eq(t, d.last("main", "status"), "ok")
	d.app.collect()
	eq(t, d.snapshots("main"), int64(1))
}

func TestPatternsBranchesAndLimits(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{
		"README.md": "r", "openspec/specs/catalog/branches/spec.md": "## Purpose\nBranches.\n### Requirement: One\n",
		"openspec/decisions/0036-branches.md": "# ADR-0036: B\n- **Status:** Accepted\n", "docs/big.md": strings.Repeat("b", 2048),
		"docs/img.png": "\x89PNG\x00\x00", "docs/drafts/x.md": "draft", "docs/z.md": "zz",
	}, "KNOWLEDGE_MAX_FILE_BYTES", "1024")
	execSQL(t, d.app.db, "INSERT INTO knowledge_patterns (node_id, include_set, include, exclude, branches) VALUES ($1, true, $2, $3, $4)",
		d.id, []string{"openspec/**", "docs/**"}, []string{"docs/drafts/**"}, []string{"release/*"})
	d.push(nil, forgefake.Branch{Name: "main", SHA: sha(1)}, forgefake.Branch{Name: "release/1", SHA: sha(2)},
		forgefake.Branch{Name: "dev", SHA: sha(3)})
	d.app.collect()
	want := "docs/big.md:too_large,docs/img.png:binary,docs/z.md:,openspec/decisions/0036-branches.md:,openspec/specs/catalog/branches/spec.md:"
	eq(t, d.files("main"), want)
	eq(t, d.last("main", "status"), "partial")
	eq(t, d.files("release/1"), want)
	eq(t, d.snapshots("dev"), int64(0), "dev is not collected")
	meta := scalar[string](t, d.app.db, "SELECT meta::text FROM knowledge_files WHERE path = 'openspec/specs/catalog/branches/spec.md' LIMIT 1")
	contains(t, meta, `"capability": "catalog/branches"`)
	contains(t, meta, `"requirements": ["One"]`)
	eq(t, scalar[string](t, d.app.db, "SELECT kind FROM knowledge_files WHERE path LIKE 'openspec/decisions/%' LIMIT 1"), "adr")
	eq(t, scalar[int64](t, d.app.db, "SELECT count(*) FROM knowledge_blobs"), int64(3))
}

func TestTheFileCountLimit(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"a.md": "a", "b.md": "b", "c.md": "c"}, "KNOWLEDGE_MAX_FILES", "2")
	execSQL(t, d.app.db, "INSERT INTO knowledge_patterns (node_id, include_set, include) VALUES ($1, true, $2)", d.id, []string{"*.md"})
	d.app.collect()
	eq(t, d.files("main"), "a.md:,b.md:,c.md:limit")
}

func TestANewCommitMakesANewSnapshotAndKnownContentIsNotDownloaded(t *testing.T) {
	t.Parallel()
	files := map[string]string{"README.md": "one", "docs/a.md": "alpha"}
	d := startDocs(t, files, "KNOWLEDGE_KEEP", "1")
	execSQL(t, d.app.db, "INSERT INTO knowledge_patterns (node_id, include_set, include) VALUES ($1, true, $2)", d.id, []string{"README.md", "docs/**"})
	d.app.collect()
	blobs := d.fake.Calls("blob")
	eq(t, blobs, 2)

	d.push(map[string]string{"README.md": "one", "docs/a.md": "alpha", "main.go": "package main"}, forgefake.Branch{Name: "main", SHA: sha(2)})
	d.app.collect()
	eq(t, d.snapshots("main"), int64(1))
	eq(t, d.last("main", "commit_sha"), sha(2))
	eq(t, d.fake.Calls("blob"), blobs, "nothing downloaded again")

	d.push(map[string]string{"README.md": "two", "docs/a.md": "alpha"}, forgefake.Branch{Name: "main", SHA: sha(3)})
	d.app.collect()
	eq(t, d.snapshots("main"), int64(2))
	eq(t, d.fake.Calls("blob"), blobs+1)
	d.push(map[string]string{"README.md": "three", "docs/a.md": "alpha"}, forgefake.Branch{Name: "main", SHA: sha(4)})
	d.app.collect()
	eq(t, d.snapshots("main"), int64(2), "KNOWLEDGE_KEEP=1 keeps the latest and one more")
}

func TestATruncatedTreeIsPartial(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "x"})
	d.repo.TreeTruncated = true
	d.push(nil)
	d.app.collect()
	eq(t, d.last("main", "status"), "partial")
	eq(t, d.last("main", "truncated"), "true")
}

func TestAFailureKeepsTheLastSnapshot(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "x"})
	d.app.collect()
	d.push(nil, forgefake.Branch{Name: "main", SHA: sha(2)})
	d.fake.RateLimitUntil(time.Now().Add(time.Hour))
	d.app.collect()
	eq(t, d.last("main", "status"), "failed")
	eq(t, d.last("main", "error_code"), "forge.rate_limited")
	eq(t, d.files("main"), "README.md:", "the last successful snapshot stays")
	next := scalar[bool](t, d.app.db, "SELECT next_run_at > now() + interval '50 minutes' FROM knowledge_settings WHERE project_id = $1", d.id)
	eq(t, next, true, "retried at the reset")
	d.fake.RateLimitUntil(time.Time{})
	d.fake.Token = "other"
	d.app.collect()
	eq(t, d.last("main", "error_code"), "forge.unauthorized")
	eq(t, scalar[int64](t, d.app.db, "SELECT count(*) FROM knowledge_snapshots WHERE status = 'failed'"), int64(1), "one failed attempt kept")
	d.fake.Token = forgeToken
	d.app.collect()
	eq(t, d.last("main", "status"), "ok")
	eq(t, d.last("main", "commit_sha"), sha(2))
}

func TestCollectNowAndRules(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "x"})
	d.app.collect()
	at := d.last("main", "collected_at")
	time.Sleep(10 * time.Millisecond)
	execSQL(t, d.app.db, "UPDATE knowledge_settings SET force = true WHERE project_id = $1", d.id)
	d.app.collect()
	if d.last("main", "collected_at") == at {
		t.Fatal("collect now did not collect")
	}
	eq(t, scalar[bool](t, d.app.db, "SELECT force FROM knowledge_settings WHERE project_id = $1", d.id), false)

	manual := d.app.nodeID(d.admin, "project", d.org, "manual")
	d.app.collect()
	eq(t, scalar[int64](t, d.app.db, "SELECT count(*) FROM knowledge_snapshots WHERE project_id = $1", manual), int64(0))
	d.fake.Delete(1)
	d.app.syncNow(d.admin, d.org, d.conn)
	execSQL(t, d.app.db, "UPDATE knowledge_settings SET force = true")
	before := d.snapshots("main")
	d.app.collect()
	eq(t, d.snapshots("main"), before, "orphaned: no new snapshots, the old ones stay")
}

func TestDeletingABranchDeletesItsSnapshots(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "x"})
	execSQL(t, d.app.db, "INSERT INTO knowledge_patterns (node_id, branches) VALUES ($1, '{feature/*}')", d.id)
	d.push(nil, forgefake.Branch{Name: "main", SHA: sha(1)}, forgefake.Branch{Name: "feature/x", SHA: sha(5)})
	d.app.collect()
	eq(t, d.snapshots("feature/x"), int64(1))
	eq(t, d.app.call("DELETE", nodePath(d.id)+"/branches/item?name=feature%2Fx", d.admin).status, 204)
	eq(t, d.snapshots("feature/x"), int64(0))
	d.push(nil, forgefake.Branch{Name: "main", SHA: sha(1)}, forgefake.Branch{Name: "feature/y", SHA: sha(6)})
	d.app.collect()
	eq(t, d.snapshots("feature/y"), int64(1))
	execSQL(t, d.app.db, "UPDATE branches SET gone_at = now() - interval '100 days' WHERE name = 'feature/y'")
	d.app.collect()
	eq(t, d.snapshots("feature/y"), int64(1), "a gone branch keeps its snapshot")
	pruneBranches(t, d.app)
	eq(t, d.snapshots("feature/y"), int64(0))
}

func TestTwoReplicasCollectOnce(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "x"})
	other := appOver(t, d.app.db)
	ids := make(chan int, 2)
	for _, a := range []*testApp{d.app, other} {
		go func() { ids <- a.collect() }()
	}
	eq(t, <-ids+<-ids, 1)
	eq(t, d.snapshots("main"), int64(1))
}
