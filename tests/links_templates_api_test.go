package tests

import (
	"encoding/base64"
	"strings"
	"testing"

	"svc-registry/internal/feature/links/uploads"
)

var tinyPNG, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")

func TestLinkTemplateTitleAndIcon(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	org := a.nodeID(admin, "organization", "", "acme")
	project := a.nodeID(admin, "project", org, "api")
	file, err := uploads.New(a.cfg.Uploads.Dir).Put(tinyPNG)
	if err != nil {
		t.Fatal(err)
	}
	put := func(key string, body obj) reply {
		body["kind_key"] = "grafana"
		body["template"] = "https://grafana.example/{project.slug}"
		return a.send("PUT", templatesPath(org)+"/"+key, admin, body)
	}

	r := put("metrics", obj{"title": "  Метрики ", "icon": obj{"file": file}})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["title"], any("Метрики"))
	eq(t, jsonText(r.json(t)["icon"]), `{"kind":"file","url":"/api/v1/link-icons/`+file+`"}`)
	r = put("board", obj{"icon": obj{"url": "https://cdn.example/g.svg"}})
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(r.json(t)["icon"]), `{"kind":"url","url":"https://cdn.example/g.svg"}`)
	r = put("plain", obj{})
	eq(t, r.json(t)["title"], nil)
	eq(t, at(r.json(t), "icon", "kind"), any("builtin"))
	eq(t, at(r.json(t), "icon", "name") != nil, true)

	items := a.links(admin, project, "")
	byKey := map[string]obj{}
	for _, it := range items {
		byKey[it.(obj)["link_key"].(string)] = it.(obj)
	}
	eq(t, byKey["metrics"]["title"], any("Метрики"), "inherited with the template")
	eq(t, at(byKey["metrics"], "icon", "kind"), any("file"))

	for _, bad := range []obj{
		{"icon": obj{"url": "http://cdn.example/g.svg"}},
		{"icon": obj{"url": "https://x", "file": file}},
		{"icon": obj{"file": strings.Repeat("a", 64) + ".png"}},
		{"icon": obj{"file": "../x.png"}},
		{"icon": obj{"path": "x"}},
		{"icon": "grafana"},
	} {
		r := put("bad", bad)
		eq(t, r.status, 400, jsonText(bad))
		eq(t, code(t, r), any("validation.invalid_link_icon"), jsonText(bad))
	}
	r = put("bad", obj{"title": strings.Repeat("x", 101)})
	eq(t, code(t, r), any("validation.invalid_link_title"))
}
