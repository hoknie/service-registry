package tests

import (
	"net/url"
	"testing"

	"svc-registry/internal/testsupport/forgefake"
)

func knowledgePath(id string) string { return nodePath(id) + "/knowledge" }

func (d *docsProject) viewer() string {
	d.app.t.Helper()
	bob := bobSession(d.app)
	d.app.grant(d.admin, d.id, "user", "bob@example.com", "viewer")
	return bob
}

func TestKnowledgeSettingsThroughTheAPI(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "x"})
	bob := d.viewer()
	r := d.app.get(knowledgePath(d.id)+"/settings", bob)
	eq(t, r.status, 200, r.text())
	defaults := `{"include":null,"exclude":[],"branches":[],"own":{},"from":{"branches":null,"exclude":null,"include":null}}`
	eq(t, r.text(), defaults)

	body := obj{"include": []string{"openspec/**", "docs/**/*.md", "README.md"}, "exclude": []string{"docs/drafts/**"}, "branches": []string{"release/*"}}
	r = d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, body)
	eq(t, r.status, 200, r.text())
	contains(t, r.text(), `"include":["openspec/**","docs/**/*.md","README.md"],"exclude":["docs/drafts/**"],"branches":["release/*"]`)
	contains(t, r.text(), `"own":{"branches":["release/*"],"exclude":["docs/drafts/**"],"include":["openspec/**","docs/**/*.md","README.md"]}`)
	eq(t, d.app.get(knowledgePath(d.id)+"/settings", bob).text(), r.text())

	r = d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"include": []string{"docs/[a-"}})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_knowledge_settings"))
	eq(t, d.app.send("PUT", knowledgePath(d.id)+"/settings", bob, body).status, 403)
	r = d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"include": nil})
	eq(t, r.text(), `{"include":null,"exclude":[],"branches":[],"own":{"include":null},"from":{"branches":null,"exclude":null,"include":null}}`)
	eq(t, d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{}).text(), defaults)

	eq(t, d.app.get(knowledgePath(d.org)+"/settings", d.admin).status, 200)
	carol := func() string {
		d.app.user("carol@example.com", false)
		return d.app.session("carol@example.com", password)
	}()
	eq(t, d.app.get(knowledgePath(d.id)+"/settings", carol).status, 404)
	eq(t, d.app.get(knowledgePath(d.id)+"/files", carol).status, 404)

	secret := d.app.pat(bob, "read")
	eq(t, d.app.bearer("GET", knowledgePath(d.id)+"/settings", secret, nil).status, 200)
	r = d.app.bearer("PUT", knowledgePath(d.id)+"/settings", secret, body)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
}

func TestCollectNowThroughTheAPI(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "x"})
	r := d.app.get(knowledgePath(d.id), d.admin)
	eq(t, r.status, 200, r.text())
	k := r.json(t)
	eq(t, k["synced"], any(true))
	b := at(k, "branches", 0).(obj)
	eq(t, b["name"], any("main"))
	eq(t, b["pending"], any(true))
	eq(t, b["snapshot"], nil)

	eq(t, d.app.call("POST", knowledgePath(d.id)+"/collect", d.admin).status, 202)
	eq(t, scalar[bool](t, d.app.db, "SELECT force FROM knowledge_settings WHERE project_id = $1", d.id), true)
	d.app.collect()
	b = at(d.app.get(knowledgePath(d.id), d.admin).json(t), "branches", 0).(obj)
	eq(t, b["pending"], any(false))
	eq(t, at(b, "snapshot", "commit"), any(sha(1)))
	eq(t, at(b, "snapshot", "status"), any("ok"))
	eq(t, at(b, "snapshot", "files"), any(float64(1)))

	bob := d.viewer()
	eq(t, d.app.call("POST", knowledgePath(d.id)+"/collect", bob).status, 403)
	manual := d.app.nodeID(d.admin, "project", d.org, "manual")
	r = d.app.call("POST", knowledgePath(manual)+"/collect", d.admin)
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.knowledge_not_synced"))
	eq(t, d.app.get(knowledgePath(manual), d.admin).json(t)["synced"], any(false))
}

func TestFilesAndOpenSpecThroughTheAPI(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{
		"README.md":                                    "# API\nhello",
		"openspec/specs/ingest/events/spec.md":         "# x\n\n## Purpose\nEvents.\n\n## Requirements\n\n### Requirement: Accept\n### Requirement: Replay\n",
		"openspec/changes/knowledge-mcp/proposal.md":   "p",
		"openspec/changes/knowledge-mcp/tasks.md":      "t",
		"openspec/changes/archive/2026-x/proposal.md":  "old",
		"openspec/decisions/0036-branches-by-name.md":  "# ADR-0036: Branches\n\n- **Status:** Accepted\n- **Supersedes:** none\n",
		"openspec/decisions/0001-behavior-in-specs.md": "# ADR-0001: Specs\nStatus: Superseded\n",
		"docs/pic.png": "\x89PNG\x00",
	})
	d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"include": []string{"README.md", "openspec/**", "docs/**"}})
	d.app.collect()
	bob := d.viewer()

	r := d.app.get(knowledgePath(d.id)+"/files", bob)
	eq(t, r.status, 200, r.text())
	files := r.json(t)
	eq(t, at(files, "snapshot", "commit"), any(sha(1)))
	eq(t, at(files, "snapshot", "status"), any("partial"))
	eq(t, len(list(t, files, "items")), 8)
	eq(t, at(files, "items", 0, "path"), any("README.md"))
	pic := at(files, "items", 1).(obj)
	eq(t, pic["path"], any("docs/pic.png"))
	eq(t, pic["skip_reason"], any("binary"))

	r = d.app.get(knowledgePath(d.id)+"/file?path=README.md", bob)
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["content"], any("# API\nhello"))
	eq(t, r.json(t)["kind"], any("doc"))
	eq(t, at(r.json(t), "snapshot", "commit"), any(sha(1)))
	r = d.app.get(knowledgePath(d.id)+"/file?path=docs%2Fpic.png", bob)
	eq(t, r.json(t)["content"], nil)
	eq(t, r.json(t)["skip_reason"], any("binary"))
	eq(t, d.app.get(knowledgePath(d.id)+"/file?path=nope.md", bob).status, 404)
	eq(t, d.app.get(knowledgePath(d.id)+"/file?path=README.md&commit="+sha(9), bob).status, 404)
	eq(t, d.app.get(knowledgePath(d.id)+"/file?path=README.md&commit="+sha(1), bob).status, 200)
	eq(t, d.app.get(knowledgePath(d.id)+"/files?branch=nope", bob).status, 404)
	r = d.app.get(knowledgePath(d.id)+"/files?branch="+url.QueryEscape("-bad"), bob)
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_branch"))

	specs := d.app.get(knowledgePath(d.id)+"/specs", bob)
	eq(t, specs.status, 200, specs.text())
	eq(t, specs.text(), `{"items":[{"capability":"ingest/events","path":"openspec/specs/ingest/events/spec.md","purpose":"Events.","requirements":["Accept","Replay"]}]}`)
	changes := d.app.get(knowledgePath(d.id)+"/changes", bob).json(t)
	eq(t, at(changes, "items", 0, "id"), any("knowledge-mcp"))
	eq(t, at(changes, "items", 0, "archived"), any(false))
	eq(t, at(changes, "items", 0, "artifacts", 1), any("tasks.md"))
	eq(t, at(changes, "items", 1, "archived"), any(true))
	adrs := d.app.get(knowledgePath(d.id)+"/adrs", bob).json(t)
	eq(t, at(adrs, "items", 0, "number"), any(float64(1)))
	eq(t, at(adrs, "items", 0, "status"), any("Superseded"))
	eq(t, at(adrs, "items", 1, "title"), any("ADR-0036: Branches"))
	eq(t, at(adrs, "items", 1, "supersedes"), any("none"))
	eq(t, d.app.get(knowledgePath(d.id)+"/specs?branch=release", bob).text(), `{"items":[]}`)

	d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"include": []string{"README.md"}, "branches": []string{"release/*"}})
	d.push(nil, forgefake.Branch{Name: "main", SHA: sha(1)}, forgefake.Branch{Name: "release/1", SHA: sha(2)})
	d.app.collect()
	r = d.app.get(knowledgePath(d.id)+"/files?branch=release%2F1", bob)
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "snapshot", "branch"), any("release/1"))
	eq(t, len(list(t, r.json(t), "items")), 1)
}
