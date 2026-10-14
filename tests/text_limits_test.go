package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"svc-registry/internal/testsupport/forgefake"
	"svc-registry/internal/testsupport/oidcfake"
)

func longRef(segments int) string {
	parts := make([]string, segments)
	for i := range parts {
		parts[i] = "segment-" + string(rune('a'+i%26))
	}
	return strings.Join(parts, "/")
}

func TestForgeSkipsRepositoriesWithTooLongValues(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/" + strings.Repeat("a", 1100)})
	longBranch := longRef(30)
	f.Put(forgefake.Repo{ID: 2, Path: "acme-inc/api", DefaultBranch: longBranch,
		Branches: []forgefake.Branch{{Name: longBranch, SHA: sha(1)}, {Name: "dev", SHA: sha(2)}}})
	conn := idOf(app.connect(root, acme, f, nil))

	run := app.syncNow(root, acme, conn)
	eq(t, run["status"], any("succeeded"), run)
	eq(t, run["created"], any(float64(1)), run)
	eq(t, at(run, "problems", 0, "code"), any("forge.value_too_long"))
	eq(t, len([]rune(at(run, "problems", 0, "full_path").(string))) <= 1024, true)
	api := app.childBySlug(root, acme, "api")
	eq(t, api["default_branch"], nil)
	branches := list(t, app.get(nodePath(idOf(api))+"/branches?state=all", root).json(t), "items")
	eq(t, len(branches), 1)
	eq(t, at(branches[0], "name"), any("dev"))

	items := list(t, app.call("POST", connectionPath(acme, conn)+"/preview", root).json(t), "items")
	codes := map[string]any{}
	for _, it := range items {
		codes[it.(obj)["action"].(string)] = it.(obj)["code"]
	}
	eq(t, codes["skip"], any("forge.value_too_long"))
}

func TestCollectionDropsTooLongBranches(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	repo := filepath.Join(a.root, "repo")
	g := newGitRepo(t, repo)
	g.commit(map[string]string{"README.md": "# Repo"})
	long := longRef(30)
	g.branch(long)
	g.commit(map[string]string{"README.md": "long"})
	a.useSource(a.project, "local_git", repo)
	a.collect()
	names := []string{}
	for _, b := range list(t, a.get(nodePath(a.project)+"/branches?state=all", a.admin).json(t), "items") {
		names = append(names, b.(obj)["name"].(string))
	}
	eq(t, strings.Contains(strings.Join(names, ","), "segment-a/"), false, names)
	eq(t, len(names) >= 1, true, names)
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM knowledge_snapshots WHERE project_id = $1 AND char_length(branch) > 255", a.project), int64(0))
}

func TestOAuthRefusesTooLongSubjectAndDropsTooLongEmail(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t)
	idp.SignIn(oidcfake.Person{Subject: strings.Repeat("s", 256), Email: "x@example.com", EmailVerified: true, Name: "X"})
	eq(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", "").header("Location"), "/login?error=auth.oauth_failed")
	eq(t, app.count("user_identities"), int64(0))

	app.user("ann@example.com", false)
	idp.SignIn(ann())
	eq(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", "").status, 302)
	person := ann()
	person.Email = strings.Repeat("a", 250) + "@example.com"
	idp.SignIn(person)
	r := app.oauthLogin("/api/v1/auth/oauth/corp/start", "")
	eq(t, r.header("Location"), "/")
	eq(t, scalar[string](t, app.db, "SELECT email FROM user_identities WHERE subject = 'sub-ann'"), "ann@example.com")
}
