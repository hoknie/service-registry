package tests

import (
	"path/filepath"
	"testing"

	"svc-registry/internal/testsupport/forgefake"
)

func TestCreateProjectWithSource(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	create := func(slug string, source any) reply {
		return a.send("POST", "/api/v1/catalog/nodes", a.admin, obj{"kind": "project", "parent_id": a.org, "slug": slug, "name": slug, "source": source})
	}
	projects := func() int64 { return scalar[int64](t, a.db, "SELECT count(*) FROM nodes WHERE kind = 'project'") }

	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# Docs")
	r := create("docs", obj{"kind": "local_dir", "path": dir})
	eq(t, r.status, 201, r.text())
	id := idOf(r.json(t))
	src := a.get(sourcePath(id), a.admin).json(t)
	eq(t, src["kind"], any("local_dir"))
	eq(t, src["path"], any(dir))
	eq(t, a.collect() >= 1, true)
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM knowledge_snapshots WHERE project_id = $1", id), int64(1))

	f := forgefake.Start(t, "gitlab", forgeToken, "platform")
	f.Put(forgefake.Repo{ID: 7, Path: "platform/api", Files: map[string]string{"README.md": "# API"},
		Branches: []forgefake.Branch{{Name: "main", SHA: sha(7)}}})
	r = create("api2", obj{"kind": "remote", "forge": "gitlab", "url": "https://gitlab.example.com/platform/api",
		"api_url": f.APIURL() + "/api/v4", "credentials": obj{"token": forgeToken}})
	eq(t, r.status, 201, r.text())
	lacks(t, r.text(), forgeToken)
	node := r.json(t)
	eq(t, node["forge"], any("gitlab"))
	eq(t, node["repo_url"], any("https://gitlab.example.com/platform/api"))
	eq(t, node["key"] != nil, true, "the project still gets its key")
	remote := idOf(node)
	a.collect()
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM knowledge_snapshots WHERE project_id = $1", remote), int64(1))

	before := projects()
	for _, c := range []struct {
		source any
		status int
		code   string
	}{
		{obj{"kind": "local_dir", "path": "/etc"}, 400, "validation.knowledge_path_not_allowed"},
		{obj{"kind": "remote", "forge": "svn", "url": "https://x.example/a/b"}, 400, "validation.invalid_knowledge_source"},
		{obj{"path": dir}, 400, "validation.invalid_body"},
	} {
		r := create("bad", c.source)
		eq(t, r.status, c.status, c.source)
		eq(t, code(t, r), any(c.code), c.source)
	}
	eq(t, projects(), before)
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM nodes WHERE slug = 'bad'"), int64(0))

	r = a.send("POST", "/api/v1/catalog/nodes", a.admin, obj{"kind": "folder", "parent_id": a.org, "slug": "f", "name": "F",
		"source": obj{"kind": "local_dir", "path": dir}})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.repo_fields_not_allowed"))
}

func TestCreateProjectWithSourceConflicts(t *testing.T) {
	t.Parallel()
	plain := startApp(t)
	admin := plain.admin()
	org := plain.nodeID(admin, "organization", "", "acme")
	create := func(source obj) reply {
		return plain.send("POST", "/api/v1/catalog/nodes", admin, obj{"kind": "project", "parent_id": org, "slug": "api", "name": "API", "source": source})
	}
	r := create(obj{"kind": "local_git", "path": "/srv/repos/api"})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.local_sources_disabled"))
	r = create(obj{"kind": "remote", "forge": "github", "url": "https://github.com/a/b", "credentials": obj{"token": "ghp_x"}})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.secrets_key_missing"))
	eq(t, scalar[int64](t, plain.db, "SELECT count(*) FROM nodes WHERE kind = 'project'"), int64(0))
}
