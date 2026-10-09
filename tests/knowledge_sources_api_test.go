package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"svc-registry/internal/testsupport/forgefake"
)

func TestSourceAPIForms(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	eq(t, a.get(sourcePath(a.project), a.admin).text(), "null")
	f := forgefake.Start(t, "gitlab", forgeToken, "platform")
	f.Put(forgefake.Repo{ID: 3, Path: "platform/api", Files: map[string]string{"README.md": "x"}})

	body := obj{"kind": "remote", "forge": "gitlab", "url": "https://gitlab.example.com/platform/api", "credentials": obj{"token": "glpat-123"}}
	r := a.send("PUT", sourcePath(a.project), a.admin, body)
	eq(t, r.status, 200, r.text())
	lacks(t, r.text(), "glpat-123")
	src := r.json(t)
	eq(t, src["api_url"], any("https://gitlab.example.com/api/v4"))
	eq(t, src["path"], nil)
	eq(t, at(src, "credentials", "mode"), any("stored"))
	fp := at(src, "credentials", "fingerprint").(string)
	eq(t, len(fp), 4)
	lacks(t, a.get(sourcePath(a.project), a.admin).text(), "glpat-123")
	enc := scalar[string](t, a.db, "SELECT credentials_enc FROM knowledge_sources")
	lacks(t, enc, "glpat-123")

	delete(body, "credentials")
	eq(t, at(a.send("PUT", sourcePath(a.project), a.admin, body).json(t), "credentials", "fingerprint"), any(fp))
	body["url"] = "https://gitlab.example.com/platform/other"
	eq(t, at(a.send("PUT", sourcePath(a.project), a.admin, body).json(t), "credentials", "mode"), any("none"))
	body["credentials"] = obj{"reference": "env:DOCS_TOKEN"}
	r = a.send("PUT", sourcePath(a.project), a.admin, body)
	eq(t, at(r.json(t), "credentials", "mode"), any("reference"))
	eq(t, at(r.json(t), "credentials", "fingerprint"), nil)

	check := obj{"kind": "remote", "forge": "gitlab", "url": "https://gitlab.example/platform/api", "api_url": f.APIURL() + "/api/v4", "credentials": nil}
	r = a.send("POST", sourcePath(a.project)+"/check", a.admin, check)
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["ok"], any(false))
	eq(t, r.json(t)["error_code"], any("forge.unauthorized"))
	check["credentials"] = obj{"token": forgeToken}
	r = a.send("POST", sourcePath(a.project)+"/check", a.admin, check)
	eq(t, r.json(t)["ok"], any(true), r.text())
	eq(t, r.json(t)["branches"], any(float64(1)))
	eq(t, at(a.get(sourcePath(a.project), a.admin).json(t), "url"), any("https://gitlab.example.com/platform/other"), "check saves nothing")

	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "x")
	r = a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_dir", "path": dir})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["kind"], any("local_dir"))
	eq(t, r.json(t)["url"], nil)
	eq(t, at(r.json(t), "credentials", "mode"), any("none"))
	r = a.send("POST", sourcePath(a.project)+"/check", a.admin, obj{"kind": "local_git", "path": dir})
	eq(t, r.json(t)["error_code"], any("source.not_a_repository"))
	eq(t, a.send("POST", sourcePath(a.project)+"/check", a.admin, obj{"kind": "local_dir", "path": dir}).json(t)["branches"], any(float64(1)))

	for _, c := range []struct {
		body obj
		code string
	}{
		{obj{"kind": "local_dir", "path": a.root + "/../etc"}, "validation.knowledge_path_not_allowed"},
		{obj{"kind": "local_dir", "path": "/etc"}, "validation.knowledge_path_not_allowed"},
		{obj{"kind": "local_dir", "path": "docs"}, "validation.invalid_knowledge_source"},
		{obj{"kind": "remote", "forge": "bitbucket", "url": "https://bitbucket.org/a/b"}, "validation.invalid_knowledge_source"},
		{obj{"kind": "remote", "forge": "github", "url": "https://github.com/a/b", "credentials": obj{}}, "validation.invalid_knowledge_source"},
	} {
		r := a.send("PUT", sourcePath(a.project), a.admin, c.body)
		eq(t, r.status, 400, c.body)
		eq(t, code(t, r), any(c.code), c.body)
	}
	eq(t, a.get(sourcePath(a.project), a.admin).json(t)["kind"], any("local_dir"), "refused bodies change nothing")

	eq(t, a.call("DELETE", sourcePath(a.project), a.admin).status, 204)
	eq(t, a.get(sourcePath(a.project), a.admin).text(), "null")
	eq(t, a.call("DELETE", sourcePath(a.project), a.admin).status, 204)
}

func TestSourceAPIRefusals(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	bob := bobSession(a.testApp)
	a.grant(a.admin, a.project, "user", "bob@example.com", "viewer")
	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "x")
	body := obj{"kind": "local_dir", "path": dir}
	eq(t, a.send("PUT", sourcePath(a.project), bob, body).status, 403)
	eq(t, a.get(sourcePath(a.project), bob).status, 200)
	eq(t, a.send("PUT", sourcePath(a.org), a.admin, body).status, 404)
	a.user("carol@example.com", false)
	a.grant(a.admin, a.project, "user", "carol@example.com", "editor")
	carol := a.session("carol@example.com", password)
	eq(t, a.bearer("GET", sourcePath(a.project), a.pat(carol, "read"), nil).status, 200)
	eq(t, a.bearer("PUT", sourcePath(a.project), a.pat(carol, "read"), body).status, 403)
	eq(t, a.bearer("PUT", sourcePath(a.project), a.pat(carol, "write"), body).status, 200)

	plain := startApp(t)
	admin := plain.admin()
	org := plain.nodeID(admin, "organization", "", "acme")
	project := plain.nodeID(admin, "project", org, "api")
	r := plain.send("PUT", sourcePath(project), admin, obj{"kind": "local_git", "path": "/srv/repos/api"})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.local_sources_disabled"))
	r = plain.send("PUT", sourcePath(project), admin, obj{"kind": "remote", "forge": "github", "url": "https://github.com/a/b",
		"credentials": obj{"token": "ghp_x"}})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.secrets_key_missing"))
	r = plain.send("PUT", sourcePath(project), admin, obj{"kind": "remote", "forge": "github", "url": "https://github.com/a/b",
		"credentials": obj{"reference": "env:GH_TOKEN"}})
	eq(t, r.status, 200, "a reference needs no key")
	eq(t, strings.Contains(r.text(), "GH_TOKEN"), false, "the reference is not answered back")
}

func TestSourceBecomesProjectLink(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	node := func() map[string]any { return a.get(nodePath(a.project), a.admin).json(t) }
	eq(t, node()["repo_url"], nil)
	r := a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "remote", "forge": "github", "url": "https://github.com/acme/api"})
	eq(t, r.status, 200, r.text())
	eq(t, node()["forge"], any("github"))
	eq(t, node()["repo_url"], any("https://github.com/acme/api"))

	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "x")
	eq(t, a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_dir", "path": dir}).status, 200)
	eq(t, node()["repo_url"], any("https://github.com/acme/api"))
	eq(t, a.call("DELETE", sourcePath(a.project), a.admin).status, 204)
	eq(t, node()["forge"], any("github"))

	d := startDocs(t, map[string]string{"README.md": "x"})
	before := d.app.get(nodePath(d.id), d.admin).json(t)["repo_url"]
	eq(t, before != nil, true, "a synced project has a link")
	r = d.app.send("PUT", sourcePath(d.id), d.admin, obj{"kind": "remote", "forge": "gitlab", "url": "https://gitlab.example.com/platform/api"})
	eq(t, r.status, 200, r.text())
	eq(t, d.app.get(nodePath(d.id), d.admin).json(t)["repo_url"], before)
	eq(t, d.app.get(sourcePath(d.id), d.admin).json(t)["url"], any("https://gitlab.example.com/platform/api"))
}

func TestSourceWorkingTreeFlag(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	repo := filepath.Join(a.root, "repo")
	newGitRepo(t, repo).commit(map[string]string{"README.md": "# Repo"})
	r := a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_git", "path": repo})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["working_tree"], any(true), "on by default")
	r = a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_git", "path": repo, "working_tree": false})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["working_tree"], any(false))
	eq(t, a.get(sourcePath(a.project), a.admin).json(t)["working_tree"], any(false), "stored")

	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# Docs")
	r = a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_dir", "path": dir, "working_tree": true})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_knowledge_source"))
	r = a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_dir", "path": dir})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["working_tree"], any(false), "other kinds report false")
}

func TestSourceIncludeIgnoredFlag(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	repo := filepath.Join(a.root, "repo")
	newGitRepo(t, repo).commit(map[string]string{"README.md": "# Repo"})
	r := a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_git", "path": repo})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["include_ignored"], any(false), "off by default")
	r = a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_git", "path": repo, "include_ignored": true})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["include_ignored"], any(true))
	eq(t, a.get(sourcePath(a.project), a.admin).json(t)["include_ignored"], any(true), "stored")
	eq(t, a.send("POST", sourcePath(a.project)+"/check", a.admin, obj{"kind": "local_git", "path": repo, "include_ignored": true}).status, 200)

	r = a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_git", "path": repo, "working_tree": false, "include_ignored": true})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_knowledge_source"))
	eq(t, a.get(sourcePath(a.project), a.admin).json(t)["include_ignored"], any(true), "unchanged")

	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# Docs")
	r = a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_dir", "path": dir, "include_ignored": true})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_knowledge_source"))
	r = a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_dir", "path": dir})
	eq(t, r.json(t)["include_ignored"], any(false), "other kinds report false")
}
