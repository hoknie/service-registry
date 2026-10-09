package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/service"
)

type scanRow struct {
	Status, Trigger, Source, Code, Warnings, Branches string
	Repeats                                           int
}

func (a *sourcesApp) scans(project, kind string) []scanRow {
	a.t.Helper()
	rows, err := a.db.Pool.Query(context.Background(), `
		SELECT status, trigger, COALESCE(source, ''), COALESCE(error_code, ''), array_to_string(warnings, ','), branches::text, repeats
		FROM knowledge_scans
		WHERE project_id = $1
			AND kind = $2
		ORDER BY started_at DESC, id DESC`, project, kind)
	if err != nil {
		a.t.Fatal(err)
	}
	defer rows.Close()
	var out []scanRow
	for rows.Next() {
		var r scanRow
		if err := rows.Scan(&r.Status, &r.Trigger, &r.Source, &r.Code, &r.Warnings, &r.Branches, &r.Repeats); err != nil {
			a.t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func (a *sourcesApp) useSource(project, kind, path string, include ...string) {
	a.t.Helper()
	r := a.send("PUT", sourcePath(project), a.admin, obj{"kind": kind, "path": path})
	eq(a.t, r.status, 200, r.text())
	eq(a.t, a.send("PUT", settingsPath(project), a.admin, obj{"include": include}).status, 200)
}

func TestScanOfALocalDirectoryCountsFilesAndSkips(t *testing.T) {
	t.Parallel()
	a := startSources(t, "KNOWLEDGE_MAX_FILE_BYTES", "1024")
	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# Docs")
	writeFile(t, filepath.Join(dir, "big.md"), strings.Repeat("x", 2048))
	a.useSource(a.project, "local_dir", dir, "**/*.md")
	a.collect()
	s := a.scans(a.project, "collect")
	eq(t, len(s), 1)
	eq(t, s[0].Status, "ok")
	eq(t, s[0].Trigger, "schedule")
	eq(t, s[0].Source, "local_dir")
	eq(t, s[0].Warnings, "collect.files_skipped")
	contains(t, s[0].Branches, `"files": 1`)
	contains(t, s[0].Branches, `"too_large": 1`)
}

func TestScanOfAGitSourceWithoutMatchingFilesWarns(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	dir := filepath.Join(a.root, "repo")
	g := newGitRepo(t, dir)
	g.commit(map[string]string{"README.md": "# Repo"})
	writeFile(t, filepath.Join(dir, "openspec", "specs", "a.md"), "not committed")
	a.useSource(a.project, "local_git", dir, "openspec/**/*.md")
	eq(t, a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_git", "path": dir, "working_tree": false}).status, 200)
	a.collect()
	s := a.scans(a.project, "collect")
	eq(t, len(s), 1)
	eq(t, s[0].Status, "warning")
	eq(t, s[0].Source, "local_git")
	eq(t, s[0].Warnings, "collect.no_files_matched")
	contains(t, s[0].Branches, `"files": 0`)
}

func TestScanOfABrokenSourceFails(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	dir := filepath.Join(a.root, "repo")
	g := newGitRepo(t, dir)
	g.commit(map[string]string{"README.md": "# Repo"})
	a.useSource(a.project, "local_git", dir)
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		t.Fatal(err)
	}
	a.collect()
	s := a.scans(a.project, "collect")
	eq(t, len(s), 1)
	eq(t, s[0].Status, "failed")
	eq(t, s[0].Code, "source.not_a_repository")
}

func TestScanHistoryCollapsesRepeatsAndKeepsTheLimit(t *testing.T) {
	t.Parallel()
	a := startSources(t, "KNOWLEDGE_SCAN_HISTORY", "3")
	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# v0")
	a.useSource(a.project, "local_dir", dir)
	a.collect()
	a.collect()
	a.collect()
	a.collect()
	s := a.scans(a.project, "collect")
	eq(t, len(s), 2, s)
	eq(t, s[0].Status, "unchanged")
	eq(t, s[0].Repeats, 2)
	eq(t, s[1].Status, "ok")

	eq(t, a.send("POST", knowledgePath(a.project)+"/collect", a.admin, nil).status, 202)
	a.collect()
	eq(t, a.scans(a.project, "collect")[0].Trigger, "manual")

	for i := 1; i <= 3; i++ {
		writeFile(t, filepath.Join(dir, "README.md"), "# v"+strings.Repeat("!", i))
		a.collect()
	}
	s = a.scans(a.project, "collect")
	eq(t, len(s), 3, "the limit")
	for _, r := range s {
		eq(t, r.Status, "ok")
	}
	execSQL(t, a.db, "INSERT INTO knowledge_scans (id, project_id, kind, trigger, status, started_at, finished_at) "+
		"VALUES (gen_random_uuid(), $1, 'index', 'schedule', 'ok', now(), now())", a.project)
	writeFile(t, filepath.Join(dir, "README.md"), "# last")
	a.collect()
	eq(t, len(a.scans(a.project, "index")), 1, "another kind is not pruned")
}

func TestScanOfAProjectWithoutSourceWarns(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	execSQL(t, a.db, "INSERT INTO knowledge_settings (project_id) VALUES ($1)", a.project)
	if err := service.RunKnowledgeCollect(context.Background(), a.state, uuid.MustParse(a.project)); err != nil {
		t.Fatal(err)
	}
	s := a.scans(a.project, "collect")
	eq(t, len(s), 1)
	eq(t, s[0].Status, "warning")
	eq(t, s[0].Warnings, "collect.no_source")
	eq(t, s[0].Source, "")
}
