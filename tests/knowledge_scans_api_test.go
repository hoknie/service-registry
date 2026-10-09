package tests

import (
	"path/filepath"
	"strings"
	"testing"
)

const scansPath = "/api/v1/knowledge/scans"

func TestKnowledgeScansAPIFiltersAndPages(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# Docs")
	a.useSource(a.project, "local_dir", dir)
	a.collect()
	other := a.nodeID(a.admin, "project", a.org, "broken")
	repo := filepath.Join(a.root, "repo")
	newGitRepo(t, repo).commit(map[string]string{"README.md": "# Repo"})
	a.useSource(other, "local_git", repo)
	execSQL(t, a.db, "UPDATE knowledge_sources SET path = $2 WHERE project_id = $1", other, filepath.Join(a.root, "gone"))
	a.collect()

	r := a.get(scansPath, a.admin)
	eq(t, r.status, 200, r.text())
	body := r.json(t)
	eq(t, body["total"], any(float64(3)))
	contains(t, r.text(), `"duration_ms":`)

	r = a.get(scansPath+"?kind=collect&status=failed,warning", a.admin)
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["total"], any(float64(1)))
	failed := at(r.json(t), "items", 0).(obj)
	eq(t, at(failed, "project", "path"), any("acme/broken"))
	eq(t, failed["status"], any("failed"))
	eq(t, at(failed, "error", "code"), any("source.not_found"))
	eq(t, failed["kind"], any("collect"))
	eq(t, failed["trigger"], any("schedule"))
	eq(t, failed["source"], any("local_git"))

	r = a.get(scansPath+"?project="+a.project+"&trigger=schedule", a.admin)
	own := list(t, r.json(t), "items")
	eq(t, len(own), 2)
	eq(t, own[0].(obj)["status"], any("unchanged"), "newest first")
	eq(t, own[1].(obj)["status"], any("ok"))
	eq(t, at(own[1], "branches", 0, "files"), any(float64(1)))
	eq(t, at(own[1], "branches", 0, "error"), nil)
	eq(t, at(own[1], "project", "id"), any(a.project))

	r = a.get(scansPath+"?limit=1&offset=1", a.admin)
	eq(t, len(list(t, r.json(t), "items")), 1)
	eq(t, r.json(t)["total"], any(float64(3)))
	r = a.get(scansPath+"?offset=10", a.admin)
	eq(t, r.json(t)["total"], any(float64(3)), "total past the end")

	for _, q := range []string{"status=broken", "kind=sync", "trigger=cron", "project=acme"} {
		r = a.get(scansPath+"?"+q, a.admin)
		eq(t, r.status, 400, q)
		eq(t, code(t, r), any("validation.invalid_scan_filter"))
	}
	r = a.get(scansPath+"?limit=0", a.admin)
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_pagination"))
}

func TestKnowledgeScansAPIIsForSuperadmins(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	bob := bobSession(a.testApp)
	r := a.get(scansPath, bob)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	r = a.bearer("GET", scansPath, a.pat(a.admin, "read"), nil)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	r = a.bearer("GET", scansPath, a.pat(a.admin, "admin"), nil)
	eq(t, r.status, 200, r.text())
	eq(t, strings.Contains(r.text(), `"items":[]`), true, r.text())
	eq(t, a.get(scansPath, "").status, 401)
}
