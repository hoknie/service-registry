package tests

import (
	"testing"

	"svc-registry/pkg/testing/forgefake"
)

func TestRemoteSourceGivesRepositoryMetadataAndDefaultBranch(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	f := forgefake.Start(t, "github", forgeToken, "acme")
	f.Put(forgefake.Repo{ID: 1, Path: "acme/api", DefaultBranch: "trunk", Topics: []string{"go"}, License: "MIT", Stars: 3,
		Readme: "# API\n", Languages: map[string]int{"Go": 1}, Files: map[string]string{"README.md": "# API\n"},
		Branches: []forgefake.Branch{{Name: "trunk", SHA: sha(1)}}})
	r := a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "remote", "forge": "github",
		"url": "https://github.example/acme/api", "api_url": f.APIURL(), "credentials": a.tokenOn(a.admin, a.org, forgeToken)})
	eq(t, r.status, 200, r.text())
	eq(t, a.get(nodePath(a.project), a.admin).json(t)["repository"], nil)
	a.collect()

	node := a.get(nodePath(a.project), a.admin).json(t)
	repo := node["repository"].(obj)
	eq(t, repo["origin"], any("source"))
	eq(t, repo["connection_id"], nil)
	eq(t, repo["full_path"], any("acme/api"))
	eq(t, at(repo, "topics", 0), any("go"))
	eq(t, repo["license"], any("MIT"))
	eq(t, repo["has_readme"], any(true))
	eq(t, at(repo, "languages", "Go"), any(float64(100)))
	eq(t, node["default_branch"], any("trunk"))
	readme := a.get(nodePath(a.project)+"/readme", a.admin)
	eq(t, readme.status, 200, readme.text())
	eq(t, readme.json(t)["markdown"], any("# API\n"))

	eq(t, a.send("PATCH", nodePath(a.project), a.admin, obj{"default_branch": "develop"}).status, 200)
	a.collect()
	eq(t, a.get(nodePath(a.project), a.admin).json(t)["default_branch"], any("develop"))

	eq(t, a.call("DELETE", sourcePath(a.project), a.admin).status, 204)
	eq(t, a.get(nodePath(a.project), a.admin).json(t)["repository"], nil)
	eq(t, a.get(nodePath(a.project)+"/readme", a.admin).status, 404)
}

func TestSourceReadmeNeedsReadAccess(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	a.user("stranger@example.com", false)
	stranger := a.session("stranger@example.com", password)
	eq(t, a.get(nodePath(a.project)+"/readme", stranger).status, 404)
}
