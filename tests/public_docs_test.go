package tests

import (
	"strings"
	"testing"

	"svc-registry/pkg/testing/hostingfake"
)

func (d *docsProject) publicGet(path string, extra ...string) reply {
	d.app.t.Helper()
	return request(d.app.t, d.app.addr, "GET", path, "", extra...)
}

func publicDocs(t *testing.T, extra ...string) (*docsProject, string) {
	t.Helper()
	d := startDocs(t, map[string]string{"README.md": "# Public API\nHow to deploy the service.", "docs/guide.md": "# Guide\ndeploy steps"}, extra...)
	eq(t, d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"include": []string{"README.md", "docs/**"},
		"refs": []obj{{"pattern": "v*", "kind": "tag"}, {"pattern": "feat/*", "kind": "branch"}}}).status, 200)
	d.push(nil, hostingfake.Branch{Name: "main", SHA: sha(1)}, hostingfake.Branch{Name: "feat/x", SHA: sha(2)})
	d.tags(tagged("v1.0.0", 3))
	d.app.collect()
	secret := d.app.nodeID(d.admin, "project", d.org, "secret")
	return d, secret
}

func TestPublicDocsShowOnlyThePublished(t *testing.T) {
	t.Parallel()
	d, secret := publicDocs(t)
	eq(t, d.publicGet("/api/public/docs/space?root=acme").status, 404)

	eq(t, d.app.send("PUT", publicationPath(d.org), d.admin, obj{"publish": true}).status, 200)
	eq(t, d.app.send("PUT", publicationPath(secret), d.admin, obj{"publish": false}).status, 200)
	r := d.publicGet("/api/public/docs/space?root=acme")
	eq(t, r.status, 200, r.text())
	eq(t, r.header("Cache-Control"), "public, max-age=60")
	body := r.json(t)
	eq(t, jsonText(body["entry"]), `{"host":null,"kind":"organization","name":"acme","title":"acme"}`)
	eq(t, jsonText(body["children"]), `[{"has_docs":true,"kind":"project","name":"api","slug":"api"}]`)
	if strings.Contains(r.text(), `"id"`) {
		t.Fatalf("ids leak: %s", r.text())
	}
	eq(t, d.publicGet("/api/public/docs/space?root=acme&path=secret").status, 404)
	eq(t, request(t, d.app.addr, "GET", "/api/public/docs/space?root=acme&path=secret", "", "cookie", d.admin).status, 404)

	space := d.publicGet("/api/public/docs/space?root=acme&path=api").json(t)
	eq(t, jsonText(at(space, "docs", "default")), `{"collected_at":`+jsonText(at(space, "docs", "default", "collected_at"))+`,"kind":"branch","name":"main","release":false}`)
	eq(t, len(at(space, "docs", "branches").([]any)), 1)
	eq(t, at(space, "docs", "branches", 0, "name"), any("main"))

	eq(t, d.app.send("PUT", publicationPath(d.org), d.admin, obj{"publish": true, "versions": []obj{{"pattern": "v*", "kind": "tag"}}, "default_branch": false}).status, 200)
	sr := d.publicGet("/api/public/docs/space?root=acme&path=api")
	space = sr.json(t)
	eq(t, at(space, "docs", "default", "name"), any("v1.0.0"), sr.text()+" "+d.app.get(knowledgePath(d.id), d.admin).text())
	eq(t, at(space, "docs", "tags", 0, "name"), any("v1.0.0"))
	eq(t, d.publicGet("/api/public/docs/tree?root=acme&path=api&branch=feat/x").status, 404)

	tree := d.publicGet("/api/public/docs/tree?root=acme&path=api&dir=docs")
	eq(t, tree.status, 200, tree.text())
	eq(t, at(tree.json(t), "files", 0, "title"), any("Guide"))
	eq(t, at(tree.json(t), "snapshot", "ref_kind"), any("tag"))
	files := d.publicGet("/api/public/docs/files?root=acme&path=api").json(t)
	eq(t, len(files["items"].([]any)), 2)
	eq(t, jsonText(body["projects"]), `[{"name":"api","path":"api"}]`)
	file := d.publicGet("/api/public/docs/file?root=acme&path=api&file=README.md")
	eq(t, file.status, 200, file.text())
	contains(t, file.text(), "How to deploy")
	for _, key := range []string{"limits", "git_status", "error_code"} {
		if strings.Contains(file.text(), `"`+key+`"`) || strings.Contains(tree.text(), `"`+key+`"`) {
			t.Fatalf("%s leaks", key)
		}
	}

	search := d.publicGet("/api/public/docs/search?root=acme&q=deploy").json(t)
	items := search["items"].([]any)
	eq(t, len(items) > 0, true)
	eq(t, jsonText(at(items[0], "project")), `{"name":"api","path":"api"}`)
	eq(t, at(items[0], "ref_kind"), any("tag"))
}

func TestPublicDocsByNameAndHost(t *testing.T) {
	t.Parallel()
	d, _ := publicDocs(t)
	eq(t, d.app.send("PUT", publicationPath(d.id), d.admin, obj{"publish": true, "name": "acme-api", "host": "docs.acme.com"}).status, 200)
	eq(t, d.publicGet("/api/public/docs/space?root=acme").status, 404)
	r := d.publicGet("/api/public/docs/space?root=acme-api")
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "node", "kind"), any("project"))

	byHost := d.publicGet("/api/public/docs/space", "host", "docs.acme.com:8080")
	eq(t, byHost.status, 200, byHost.text())
	eq(t, at(byHost.json(t), "entry", "host"), any("docs.acme.com"))
	eq(t, d.publicGet("/api/v1/catalog/tree", "host", "docs.acme.com").status, 404)
	page := d.publicGet("/docs/guide", "host", "docs.acme.com", "accept-language", "ru")
	eq(t, page.status != 404, true, page.text())
	eq(t, strings.HasPrefix(page.header("Content-Type"), "text/html"), true)
}

func TestPublicDocsRateLimit(t *testing.T) {
	t.Parallel()
	d, _ := publicDocs(t, "DOCS_PUBLIC_RATE_PER_MIN", "10")
	var last reply
	for range 11 {
		last = d.publicGet("/api/public/docs/space?root=nope")
	}
	eq(t, last.status, 429)
	eq(t, code(t, last), any("rate_limited"))
	if last.header("Retry-After") == "" {
		t.Fatal("Retry-After")
	}
}

func TestPublicDocsAPIDescriptions(t *testing.T) {
	t.Parallel()
	d, _ := startAPIDocs(t)
	path := "/api/public/docs/api?root=acme&path=api&file=shop/v1/item.proto"
	eq(t, d.publicGet(path).status, 404)

	eq(t, d.app.send("PUT", publicationPath(d.org), d.admin, obj{"publish": true}).status, 200)
	r := d.publicGet(path)
	eq(t, r.status, 200, r.text())
	eq(t, r.header("Cache-Control"), "public, max-age=60")
	body := r.json(t)
	eq(t, body["format"], any("proto"))
	eq(t, body["path"], any("shop/v1/item.proto"))
	eq(t, at(body, "snapshot", "branch"), any("main"))
	eq(t, len(list(t, body, "messages")) > 0, true)
	for _, key := range []string{`"id":`, `"limits":`, `"git_status":`} {
		if strings.Contains(r.text(), key) {
			t.Fatalf("%s leaks: %s", key, r.text())
		}
	}
	openapi := d.publicGet("/api/public/docs/api?root=acme&path=api&file=api/openapi.yaml")
	eq(t, openapi.status, 200, openapi.text())
	eq(t, len(list(t, openapi.json(t), "operations")) > 0, true)
	eq(t, d.publicGet("/api/public/docs/api?root=acme&path=api&file=README.md").status, 404)
	eq(t, d.publicGet("/api/public/docs/api?root=acme&path=api&file=shop/v1/item.proto&branch=nope").status, 404)
}
