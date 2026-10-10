package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"svc-registry/internal/feature/links"
	"svc-registry/internal/feature/links/linkcheck"
)

const linkKindsPath = "/api/v1/link-kinds"

func allNames(en string) obj {
	return obj{"en": en, "es": en + " es", "ru": en + " ru", "zh": en + " zh"}
}

func templatesPath(node string) string { return nodePath(node) + "/link-templates" }

func linksPath(node string) string { return nodePath(node) + "/links" }

func (a *testApp) putTemplate(cookie, node, key, template string) obj {
	a.t.Helper()
	r := a.send("PUT", templatesPath(node)+"/"+key, cookie, obj{"kind_key": key, "template": template})
	if r.status != 200 {
		a.t.Fatalf("put template %s: %d %s", key, r.status, r.body)
	}
	return r.json(a.t)
}

func (a *testApp) links(cookie, project, query string) []any {
	a.t.Helper()
	r := a.get(linksPath(project)+query, cookie)
	if r.status != 200 {
		a.t.Fatalf("links: %d %s", r.status, r.body)
	}
	return list(a.t, r.json(a.t), "items")
}

type anyIP struct{}

func (anyIP) Allows(string, netip.Addr) bool { return true }

type switchChecker struct{ current atomic.Pointer[links.Checker] }

func (s *switchChecker) set(c links.Checker) { s.current.Store(&c) }

func (s *switchChecker) Check(ctx context.Context, url string) links.Result {
	return (*s.current.Load()).Check(ctx, url)
}

func (a *testApp) allowLoopbackChecks() {
	a.checker.set(linkcheck.NewWithPolicy(a.cfg.Outbound, 2*time.Second, anyIP{}))
}

type target struct {
	*httptest.Server
	hits atomic.Int32
}

func newTarget(t testing.TB, status int) *target {
	t.Helper()
	tg := &target{}
	tg.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		tg.hits.Add(1)
		w.WriteHeader(status)
	}))
	t.Cleanup(tg.Close)
	return tg
}

func (a *testApp) deployWith(project, secret, key, service, env string, extra obj) {
	a.t.Helper()
	body := deployed(key, service, env, "1.0.0", time.Now().Add(-time.Minute).UTC().Format(time.RFC3339))
	for k, v := range extra {
		body["payload"].(obj)[k] = v
	}
	if r := a.ingest(project, secret, body); r.status != 201 {
		a.t.Fatalf("ingest: %d %s", r.status, r.body)
	}
}
