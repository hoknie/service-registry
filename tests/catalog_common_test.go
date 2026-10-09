package tests

import (
	"strings"
	"testing"
)

func (a *testApp) createNode(cookie string, body obj) reply {
	return a.send("POST", "/api/v1/catalog/nodes", cookie, body)
}

func (a *testApp) node(cookie, kind, parent, slug string) obj {
	a.t.Helper()
	body := obj{"kind": kind, "parent_id": nil, "slug": slug, "name": slug}
	if parent != "" {
		body["parent_id"] = parent
	}
	r := a.createNode(cookie, body)
	if r.status != 201 {
		a.t.Fatalf("create %s: %d %s", slug, r.status, r.body)
	}
	return r.json(a.t)
}

func (a *testApp) nodeID(cookie, kind, parent, slug string) string {
	a.t.Helper()
	return idOf(a.node(cookie, kind, parent, slug))
}

func (a *testApp) grant(cookie, node, kind, subject, role string) obj {
	a.t.Helper()
	r := a.send("POST", "/api/v1/catalog/nodes/"+node+"/bindings", cookie,
		obj{"subject_kind": kind, "subject": subject, "role": role})
	if r.status != 200 {
		a.t.Fatalf("grant %s to %s: %d %s", role, subject, r.status, r.body)
	}
	return r.json(a.t)
}

func nodePath(id string) string { return "/api/v1/catalog/nodes/" + id }

func idOf(v obj) string { return v["id"].(string) }

func at(v any, path ...any) any {
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, ok := v.(obj)
			if !ok {
				return nil
			}
			v = m[k]
		case int:
			s, ok := v.([]any)
			if !ok || k >= len(s) {
				return nil
			}
			v = s[k]
		}
	}
	return v
}

func list(t testing.TB, v any, path ...any) []any {
	t.Helper()
	s, ok := at(v, path...).([]any)
	if !ok {
		t.Fatalf("no array at %v in %v", path, v)
	}
	return s
}

func has(v obj, key string) bool {
	_, ok := v[key]
	return ok
}

func slugs(t testing.TB, v obj, field string) string {
	t.Helper()
	var out []string
	for _, n := range list(t, v, field) {
		out = append(out, n.(obj)["slug"].(string))
	}
	return strings.Join(out, ",")
}

func (a *testApp) count(table string) int64 {
	a.t.Helper()
	return scalar[int64](a.t, a.db, "SELECT count(*) FROM "+table)
}

func ingestPath(project string) string {
	return "/api/v1/ingest/projects/" + project + "/events"
}

func deployed(key, service, env, version, at string) obj {
	return obj{
		"type": "service.deployed", "version": 1, "occurred_at": at, "idempotency_key": key,
		"payload": obj{"service": service, "environment": env, "version": version},
	}
}

func (a *testApp) ingest(project, secret string, body any) reply {
	a.t.Helper()
	return a.ingestWith(project, jsonText(body), "authorization", "Bearer "+secret)
}

func (a *testApp) ingestWith(project, body string, headers ...string) reply {
	a.t.Helper()
	return request(a.t, a.addr, "POST", ingestPath(project), body,
		append([]string{"content-type", "application/json"}, headers...)...)
}

func (a *testApp) projectWithKey(admin, slug string) (string, string) {
	a.t.Helper()
	org := a.nodeID(admin, "organization", "", slug+"-org")
	p := a.node(admin, "project", org, slug)
	return idOf(p), at(p, "key", "secret").(string)
}
