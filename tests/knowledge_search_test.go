package tests

import (
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func searchPath(q string, extra ...string) string {
	v := url.Values{"q": {q}}
	for i := 0; i+1 < len(extra); i += 2 {
		v.Set(extra[i], extra[i+1])
	}
	return "/api/v1/knowledge/search?" + v.Encode()
}

func matched(t *testing.T, item obj) []string {
	var out []string
	for _, s := range list(t, item, "snippet") {
		if s.(obj)["match"] == true {
			out = append(out, strings.ToLower(s.(obj)["text"].(string)))
		}
	}
	return out
}

func TestSearchFindsTextAndPathsOfVisibleProjects(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{
		"README.md":                            "# API\nThe service keeps an idempotency key for every event.",
		"openspec/specs/ingest/events/spec.md": "Events carry an idempotency key; replays are answered 200.",
		"openspec/decisions/0001-x.md":         "# ADR-0001\nnothing here",
		"docs/other.md":                        "unrelated words",
	})
	d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"include": []string{"README.md", "openspec/**", "docs/**"}})
	d.app.collect()
	bob := d.viewer()

	r := d.app.get(searchPath("idempotency key"), bob)
	eq(t, r.status, 200, r.text())
	items := list(t, r.json(t), "items")
	eq(t, len(items), 2)
	first := items[0].(obj)
	eq(t, at(first, "project", "id"), any(d.id))
	eq(t, at(first, "project", "path"), any("acme/api"))
	eq(t, first["branch"], any("main"))
	eq(t, first["commit"], any(sha(1)))
	m := matched(t, first)
	eq(t, len(m) >= 2 && contains2(m, "idempotency") && contains2(m, "key"), true, m)
	eq(t, r.json(t)["next_cursor"], nil)

	r = d.app.get(searchPath("decisions", "path", "openspec/"), bob)
	items = list(t, r.json(t), "items")
	eq(t, len(items), 1, r.text())
	eq(t, items[0].(obj)["path"], any("openspec/decisions/0001-x.md"))
	eq(t, items[0].(obj)["kind"], any("adr"))
	eq(t, len(list(t, d.app.get(searchPath("idempotency", "path", "docs/"), bob).json(t), "items")), 0)

	r = d.app.get(searchPath("idempotency", "limit", "1"), bob)
	eq(t, len(list(t, r.json(t), "items")), 1)
	next := r.json(t)["next_cursor"].(string)
	r = d.app.get(searchPath("idempotency", "limit", "1", "cursor", next), bob)
	eq(t, len(list(t, r.json(t), "items")), 1)
	eq(t, r.json(t)["next_cursor"], nil)

	d.app.user("carol@example.com", false)
	carol := d.app.session("carol@example.com", password)
	eq(t, len(list(t, d.app.get(searchPath("idempotency"), carol).json(t), "items")), 0)
	eq(t, len(list(t, d.app.get(searchPath("idempotency", "project", d.id), carol).json(t), "items")), 0)
	eq(t, len(list(t, d.app.get(searchPath("idempotency", "project", d.id), bob).json(t), "items")), 2)
	eq(t, len(list(t, d.app.get(searchPath("idempotency"), d.admin).json(t), "items")), 2)
	eq(t, len(list(t, d.app.get(searchPath("idempotency", "branch", "release/1"), d.admin).json(t), "items")), 0)
}

func TestSearchRefusals(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	for _, p := range []string{"/api/v1/knowledge/search?q=", "/api/v1/knowledge/search?q=" + strings.Repeat("a", 201),
		searchPath("x", "limit", "0"), searchPath("x", "limit", "x"), searchPath("x", "cursor", "nope"), searchPath("x", "project", "nope")} {
		r := app.get(p, admin)
		eq(t, r.status, 400, p)
		eq(t, code(t, r), any("validation.invalid_search"), p)
	}
	r := app.get(searchPath("x"), "")
	eq(t, r.status, 401)
	eq(t, code(t, r), any("auth.unauthenticated"))
	eq(t, app.bearer("GET", searchPath("x"), app.pat(admin, "read"), nil).status, 200)
	r = app.bearer("GET", searchPath("x"), app.pat(admin, "mcp"), nil)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
}

func contains2(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestSearchMatchesWordFormsByStem(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{
		"README.md":      "# API\nВетка по умолчанию собирается всегда.",
		"docs/deploy.md": "Deployments are recorded for the ADR-0036 rules.",
		"docs/both.md":   "Deployments of the ветка main.",
	})
	d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"include": []string{"README.md", "docs/**"}})
	d.app.collect()
	bob := d.viewer()
	paths := func(q string) string {
		t.Helper()
		var out []string
		for _, it := range list(t, d.app.get(searchPath(q), bob).json(t), "items") {
			out = append(out, it.(obj)["path"].(string))
		}
		slices.Sort(out)
		return strings.Join(out, ",")
	}

	r := d.app.get(searchPath("ветки", "path", "README"), bob)
	items := list(t, r.json(t), "items")
	eq(t, len(items), 1, r.text())
	eq(t, contains2(matched(t, items[0].(obj)), "ветка"), true, matched(t, items[0].(obj)))
	eq(t, paths("ветки"), "README.md,docs/both.md")
	eq(t, paths("deploy"), "docs/both.md,docs/deploy.md")
	eq(t, paths("the"), "docs/both.md,docs/deploy.md", "a query of stop words matches exact forms")
	eq(t, paths("ADR-0036"), "docs/deploy.md")
	eq(t, paths("deploy -ветки"), "docs/deploy.md", "an exclusion excludes every form")
	eq(t, paths(`"ветка по умолчанию"`), "README.md")
}

func TestSearchFindsLocalSourcesAndWordBeginnings(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# Документация\nРепозиторий хранит настройки.")
	writeFile(t, filepath.Join(dir, "deploy.md"), "Deployments are recorded; репозиторий рядом.")
	writeFile(t, filepath.Join(dir, "plain.md"), "Deployments only.")
	eq(t, a.send("PUT", sourcePath(a.project), a.admin, obj{"kind": "local_dir", "path": dir}).status, 200)
	eq(t, a.send("PUT", settingsPath(a.project), a.admin, obj{"include": []string{"*.md"}}).status, 200)
	a.collect()
	paths := func(q string) string {
		t.Helper()
		r := a.get(searchPath(q), a.admin)
		eq(t, r.status, 200, r.text())
		var out []string
		for _, it := range list(t, r.json(t), "items") {
			out = append(out, it.(obj)["path"].(string))
		}
		slices.Sort(out)
		return strings.Join(out, ",")
	}

	eq(t, paths("репозиторий"), "README.md,deploy.md", "the local branch is the default one")
	eq(t, paths("Репозит"), "README.md,deploy.md", "a word beginning")
	r := a.get(searchPath("Репозит", "path", "README"), a.admin)
	m := matched(t, list(t, r.json(t), "items")[0].(obj))
	eq(t, contains2(m, "репозиторий"), true, m)
	eq(t, paths("deploy -репозит"), "plain.md", "an exclusion by its beginning")
	eq(t, paths(`"репозиторий хранит"`), "README.md", "a phrase stays exact")
}
