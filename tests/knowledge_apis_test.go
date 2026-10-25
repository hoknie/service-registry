package tests

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"svc-registry/pkg/testing/forgefake"
)

func apiFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("../internal/feature/apis/types/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func startAPIDocs(t *testing.T) (*docsProject, string) {
	t.Helper()
	big := "{\"openapi\":\"3.0.0\",\"x\":\"" + strings.Repeat("a", 4096) + "\"}"
	d := startDocs(t, map[string]string{
		"README.md":                                "# Pets",
		"api/openapi.yaml":                         apiFixture(t, "petstore30.yaml"),
		"api/schemas/pet.yaml":                     apiFixture(t, "pet.yaml"),
		"api/schemas/owner.yaml":                   apiFixture(t, "owner.yaml"),
		"openapi.json":                             big,
		"swagger.yaml":                             apiFixture(t, "swagger.yaml"),
		"deploy/values.yaml":                       "replicas: 2\n",
		"proto/pets/v1/pets.proto":                 apiFixture(t, "pets.proto"),
		"third_party/google/api/annotations.proto": apiFixture(t, "annotations.proto"),
		"third_party/google/api/http.proto":        apiFixture(t, "http.proto"),
		"shop/v1/item.proto":                       apiFixture(t, "money.proto"),
	}, "KNOWLEDGE_MAX_FILE_BYTES", "3072")
	d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"include": []string{"**"}})
	d.app.collect()
	return d, d.viewer()
}

func TestAPIDescriptionsAreCollectedByDefault(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{
		"README.md":                         "# Pets",
		"docs/guide.md":                     "# Guide",
		"api/openapi.yaml":                  apiFixture(t, "petstore30.yaml"),
		"swagger.json":                      "{}",
		"proto/pets/v1/pets.proto":          apiFixture(t, "pets.proto"),
		"third_party/google/api/http.proto": apiFixture(t, "http.proto"),
		"apidocs/index.md":                  "# API",
		"svc/Api-Docs/v1/notes.txt":         "notes",
		"contracts/v1.yaml":                 "x: 1\n",
		"generated/openapi.yaml":            apiFixture(t, "petstore30.yaml"),
	})
	d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"exclude": []string{"generated/**"}})
	d.app.collect()
	eq(t, d.files("main"), "api/openapi.yaml:,README.md:,svc/Api-Docs/v1/notes.txt:")
	bob := d.viewer()
	body := d.app.get(knowledgePath(d.id)+"/apis", bob).json(t)
	eq(t, len(list(t, body, "items")), 1)

	r := d.app.send("PUT", knowledgePath(d.org)+"/settings", d.admin, obj{"api_paths": []string{"**/*.proto", "**/openapi*.{yaml,yml,json}", "contracts/**"}})
	eq(t, r.status, 200, r.text())
	d.push(nil, forgefake.Branch{Name: "main", SHA: sha(2)})
	d.app.collect()
	eq(t, d.files("main"), "api/openapi.yaml:,contracts/v1.yaml:,proto/pets/v1/pets.proto:,README.md:,third_party/google/api/http.proto:")
	body = d.app.get(knowledgePath(d.id)+"/apis", bob).json(t)
	eq(t, len(list(t, body, "items")), 2)
	eq(t, body["hidden_vendored"], any(float64(1)))

	eq(t, d.app.send("PUT", knowledgePath(d.id)+"/settings", d.admin, obj{"apis": false}).status, 200)
	d.push(nil, forgefake.Branch{Name: "main", SHA: sha(3)})
	d.app.collect()
	eq(t, d.files("main"), "README.md:")
}

func TestAPIDescriptionsThroughTheAPI(t *testing.T) {
	t.Parallel()
	d, bob := startAPIDocs(t)

	r := d.app.get(knowledgePath(d.id)+"/apis", bob)
	eq(t, r.status, 200, r.text())
	body := r.json(t)
	items := list(t, body, "items")
	paths := []string{}
	for _, it := range items {
		paths = append(paths, it.(obj)["path"].(string))
	}
	eq(t, strings.Join(paths, ","), "api/openapi.yaml,openapi.json,proto/pets/v1/pets.proto,shop/v1/item.proto,swagger.yaml")
	eq(t, body["hidden_vendored"], any(float64(2)))
	eq(t, at(body, "snapshot", "commit"), any(sha(1)))
	all := d.app.get(knowledgePath(d.id)+"/apis?all=1", bob).json(t)
	eq(t, len(list(t, all, "items")), 7)
	eq(t, all["hidden_vendored"], any(float64(0)))
	eq(t, at(all, "items", 5, "path"), any("third_party/google/api/annotations.proto"))
	eq(t, at(all, "items", 5, "vendored"), any(true))
	eq(t, at(all, "items", 0, "vendored"), any(false))
	open := items[0].(obj)
	eq(t, open["format"], any("openapi"))
	eq(t, open["title"], any("Petstore"))
	eq(t, open["operations"], any(float64(3)))
	eq(t, open["services"], nil)
	eq(t, open["problems"], any(float64(2)))
	eq(t, items[1].(obj)["skip_reason"], any("too_large"))
	eq(t, items[1].(obj)["operations"], nil)
	proto := items[2].(obj)
	eq(t, proto["format"], any("proto"))
	eq(t, proto["spec_version"], any("proto3"))
	eq(t, proto["services"], any(float64(1)))
	eq(t, proto["methods"], any(float64(3)))
	eq(t, proto["operations"], nil)
	eq(t, items[4].(obj)["format"], any("swagger"))

	r = d.app.get(knowledgePath(d.id)+"/api?path="+url.QueryEscape("api/openapi.yaml"), bob)
	eq(t, r.status, 200, r.text())
	doc := r.json(t)
	eq(t, at(doc, "snapshot", "commit"), any(sha(1)))
	eq(t, at(doc, "operations", 1, "key"), any("POST /pets"))
	eq(t, at(doc, "operations", 1, "request_body", "content", 0, "schema", "ref"), any("Pet"))
	eq(t, at(doc, "operations", 2, "responses", 0, "content", 0, "schema", "properties", "owner", "format"), any("email"))
	eq(t, at(doc, "operations", 2, "responses", 1, "content", 0, "schema", "unresolved"), any(true))
	eq(t, at(doc, "problems", 0, "code"), any("api.ref_unresolved"))
	eq(t, at(doc, "schemas", 0, "name"), any("Pet"))

	sw := d.app.get(knowledgePath(d.id)+"/api?path=swagger.yaml", bob).json(t)
	eq(t, at(sw, "servers", 0, "url"), any("https://api.example/v1"))
	eq(t, at(sw, "operations", 0, "request_body", "content", 0, "media_type"), any("application/json"))

	pr := d.app.get(knowledgePath(d.id)+"/api?path="+url.QueryEscape("proto/pets/v1/pets.proto"), bob).json(t)
	eq(t, at(pr, "services", 0, "methods", 0, "http", "path"), any("/v1/pets/{id}"))
	eq(t, at(pr, "services", 0, "methods", 2, "server_streaming"), any(true))
	eq(t, len(list(t, pr, "problems")), 0)

	item := d.app.get(knowledgePath(d.id)+"/api?path="+url.QueryEscape("shop/v1/item.proto"), bob).json(t)
	eq(t, at(item, "problems", 0, "code"), any("api.import_not_found"))
	eq(t, at(item, "messages", 0, "fields", 0, "type"), any("common.v1.Money"))

	skipped := d.app.get(knowledgePath(d.id)+"/api?path=openapi.json", bob)
	eq(t, skipped.status, 200, skipped.text())
	eq(t, at(skipped.json(t), "problems", 0, "code"), any("api.file_skipped"))
	eq(t, len(list(t, skipped.json(t), "operations")), 0)

	eq(t, code(t, d.app.get(knowledgePath(d.id)+"/api?path=README.md", bob)), any("not_found"))
	eq(t, d.app.get(knowledgePath(d.id)+"/api?path="+url.QueryEscape("deploy/values.yaml"), bob).status, 404)
	eq(t, d.app.get(knowledgePath(d.id)+"/api?path=nope.yaml", bob).status, 404)
	eq(t, d.app.get(knowledgePath(d.id)+"/apis?commit="+sha(9), bob).status, 404)
	r = d.app.get(knowledgePath(d.id)+"/apis?branch="+url.QueryEscape("-bad"), bob)
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_branch"))
	eq(t, d.app.call("POST", knowledgePath(d.id)+"/apis", bob).status, 405)

	d.app.user("stranger@example.com", false)
	stranger := d.app.session("stranger@example.com", password)
	eq(t, code(t, d.app.get(knowledgePath(d.id)+"/apis", stranger)), any("not_found"))
	eq(t, d.app.bearer("GET", knowledgePath(d.id)+"/apis", d.app.pat(bob, "read"), nil).status, 200)
}

func TestAPIDescriptionsThroughMCP(t *testing.T) {
	t.Parallel()
	d, bob := startAPIDocs(t)
	s := d.app.mcpSession(d.app.pat(bob, "mcp"))

	out, isErr := call(t, s, "list_apis", map[string]any{"project": d.id})
	eq(t, isErr, false, out)
	contains(t, out, `"path": "api/openapi.yaml"`)
	contains(t, out, `"methods": 3`)
	eq(t, strings.Contains(out, "third_party/"), false, out)
	out, _ = call(t, s, "list_apis", map[string]any{"project": d.id, "all": true})
	contains(t, out, `"path": "third_party/google/api/http.proto"`)

	out, isErr = call(t, s, "get_api", map[string]any{"project": d.id, "path": "api/openapi.yaml"})
	eq(t, isErr, false, out)
	contains(t, out, "# Petstore 1.0.0")
	contains(t, out, "`GET /pets/{id}`")
	contains(t, out, "api.ref_unresolved")

	out, isErr = call(t, s, "get_api", map[string]any{"project": d.id, "path": "api/openapi.yaml", "item": "POST /pets"})
	eq(t, isErr, false, out)
	contains(t, out, "## Request body")
	contains(t, out, "### Pet")
	eq(t, strings.Contains(out, "GET /pets/{id}"), false, out)

	out, isErr = call(t, s, "get_api", map[string]any{"project": d.id, "path": "proto/pets/v1/pets.proto", "item": "pets.v1.Pet"})
	eq(t, isErr, false, out)
	contains(t, out, "# message pets.v1.Pet")

	out, isErr = call(t, s, "get_api", map[string]any{"project": d.id, "path": "api/openapi.yaml", "item": "DELETE /nope"})
	eq(t, isErr, true, out)
	contains(t, out, "not_found")
}
