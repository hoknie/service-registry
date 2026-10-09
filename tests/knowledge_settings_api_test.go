package tests

import (
	"path/filepath"
	"testing"
)

func settingsPath(id string) string { return nodePath(id) + "/knowledge/settings" }

func TestSettingsAreInheritedPerField(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	folder := a.nodeID(a.admin, "folder", a.org, "backend")
	api := a.nodeID(a.admin, "project", folder, "svc")

	r := a.send("PUT", settingsPath(folder), a.admin, obj{"include": []string{"docs/**"}, "exclude": []string{"docs/drafts/**"}})
	eq(t, r.status, 200, r.text())
	contains(t, r.text(), `"own":{"exclude":["docs/drafts/**"],"include":["docs/**"]}`)
	eq(t, a.send("PUT", settingsPath(api), a.admin, obj{"exclude": []string{}}).status, 200)

	got := a.get(settingsPath(api), a.admin)
	contains(t, got.text(), `"include":["docs/**"],"exclude":[],"branches":[],"own":{"exclude":[]}`)
	s := got.json(t)
	eq(t, at(s, "from", "include", "id"), any(folder))
	eq(t, at(s, "from", "include", "kind"), any("folder"))
	eq(t, at(s, "from", "include", "name"), any("backend"))
	eq(t, at(s, "from", "exclude"), nil, "set on the project itself")
	eq(t, at(s, "from", "branches"), nil, "the default")

	r = a.send("PUT", settingsPath(api), a.admin, obj{"include": nil})
	contains(t, r.text(), `"include":null,"exclude":["docs/drafts/**"]`)
	eq(t, at(r.json(t), "from", "exclude", "name"), any("backend"))

	r = a.send("PUT", settingsPath(api), a.admin, obj{})
	contains(t, r.text(), `"include":["docs/**"],"exclude":["docs/drafts/**"],"branches":[],"own":{}`)
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM knowledge_patterns WHERE node_id = $1", api), int64(0), "an empty row is deleted")

	for _, body := range []obj{{"exclude": nil}, {"branches": nil}, {"include": []string{"docs/[a-"}}} {
		r = a.send("PUT", settingsPath(folder), a.admin, body)
		eq(t, r.status, 400, body)
		eq(t, code(t, r), any("validation.invalid_knowledge_settings"))
	}
	contains(t, a.get(settingsPath(folder), a.admin).text(), `"own":{"exclude":["docs/drafts/**"],"include":["docs/**"]}`)
}

func TestSettingsOfAFolderQueueAndShapeItsProjects(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	folder := a.nodeID(a.admin, "folder", a.org, "backend")
	withSource := a.nodeID(a.admin, "project", folder, "docs")
	bare := a.nodeID(a.admin, "project", folder, "bare")
	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# Readme")
	writeFile(t, filepath.Join(dir, "docs", "guide.md"), "# Guide")
	eq(t, a.send("PUT", sourcePath(withSource), a.admin, obj{"kind": "local_dir", "path": dir}).status, 200)
	a.collect()
	eq(t, a.fileList(withSource), "README.md")

	execSQL(t, a.db, "UPDATE knowledge_settings SET next_run_at = now() + interval '1 hour'")
	eq(t, a.send("PUT", settingsPath(folder), a.admin, obj{"include": []string{"docs/**"}}).status, 200)
	eq(t, scalar[bool](t, a.db, "SELECT next_run_at <= now() FROM knowledge_settings WHERE project_id = $1", withSource), true)
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM knowledge_settings WHERE project_id = $1", bare), int64(0))

	a.collect()
	eq(t, a.fileList(withSource), "docs/guide.md")

	bob := bobSession(a.testApp)
	a.grant(a.admin, folder, "user", "bob@example.com", "viewer")
	eq(t, a.get(settingsPath(folder), bob).status, 200)
	r := a.send("PUT", settingsPath(folder), bob, obj{})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
}

func (a *testApp) fileList(project string) string {
	a.t.Helper()
	return scalar[string](a.t, a.db, "SELECT COALESCE(string_agg(f.path, ',' ORDER BY f.path), '') FROM knowledge_files f "+
		"WHERE f.snapshot_id = (SELECT id FROM knowledge_snapshots WHERE project_id = $1 ORDER BY collected_at DESC, id DESC LIMIT 1)", project)
}
