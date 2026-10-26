package tests

import (
	"context"
	"sync/atomic"
	"testing"

	svcapp "svc-registry/internal/app"
	"svc-registry/internal/platform/metrics"
)

func TestMetricsViewIsOffWithoutMetrics(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	eq(t, jsonText(app.get("/api/v1/auth/me", admin).json(t)["features"]), `{"metrics":false}`)
	for _, path := range []string{"/api/v1/system/metrics", "/api/v1/system/metrics/family?name=go_goroutines"} {
		r := app.get(path, admin)
		eq(t, r.status, 404, path)
		eq(t, code(t, r), any("not_found"))
	}
}

func TestMetricsViewServesReadyValues(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	probe := metrics.SampleSource("probe", func(context.Context) ([]metrics.Sample, error) {
		calls.Add(1)
		return []metrics.Sample{{Name: "svcr_catalog_nodes", Labels: map[string]string{"kind": "probe"}, Value: 7}}, nil
	})
	app := startAppWith(t, []svcapp.Option{svcapp.WithMetricsSource(probe)}, "METRICS_ENABLED", "true", "METRICS_REFRESH_SECS", "30")
	admin := app.admin()
	eq(t, jsonText(app.get("/api/v1/auth/me", admin).json(t)["features"]), `{"metrics":true}`)
	app.services.Sampler.SampleNow(context.Background())
	eq(t, app.call("GET", "/api/health", "").status, 200)

	r := app.get("/api/v1/system/metrics", admin)
	eq(t, r.status, 200, r.text())
	eq(t, r.header("Cache-Control"), "no-store")
	v := r.json(t)
	eq(t, v["refresh_secs"], any(30.0))
	runtime := v["runtime"].(obj)
	if g, ok := runtime["go_goroutines"].(float64); !ok || g <= 0 {
		t.Fatalf("goroutines %v", runtime["go_goroutines"])
	}
	if _, ok := runtime["process_max_fds"]; !ok {
		t.Fatal("runtime names must always be present")
	}
	if _, ok := v["sampled_at"].(obj)["runtime"]; !ok {
		t.Fatalf("sampled_at %v", v["sampled_at"])
	}
	found := false
	for _, f := range v["families"].([]any) {
		if f.(obj)["name"] == "svcr_http_requests_total" && f.(obj)["type"] == "counter" {
			found = true
		}
	}
	eq(t, found, true)

	f := app.get("/api/v1/system/metrics/family?name=svcr_http_requests_total", admin)
	eq(t, f.status, 200, f.text())
	health := false
	for _, s := range f.json(t)["series"].([]any) {
		labels := s.(obj)["labels"].(obj)
		if labels["route"] == "/api/health" && labels["method"] == "GET" && labels["status"] == "200" && s.(obj)["value"].(float64) >= 1 {
			health = true
		}
	}
	eq(t, health, true, f.text())

	h := app.get("/api/v1/system/metrics/family?name=svcr_http_request_duration_seconds", admin).json(t)
	eq(t, h["type"], any("histogram"))
	for _, s := range h["series"].([]any) {
		buckets := s.(obj)["buckets"].([]any)
		last := buckets[len(buckets)-1].(obj)
		eq(t, last["le"], any("+Inf"))
		eq(t, last["count"], s.(obj)["count"])
	}

	for range 20 {
		eq(t, app.get("/api/v1/system/metrics/family?name=svcr_catalog_nodes", admin).status, 200)
		eq(t, app.get("/api/v1/system/metrics", admin).status, 200)
	}
	eq(t, calls.Load(), int32(1))

	missing := app.get("/api/v1/system/metrics/family?name=no_such_metric", admin)
	eq(t, missing.status, 404)
	eq(t, code(t, missing), any("not_found"))
	eq(t, app.get("/api/v1/system/metrics/family", admin).status, 404)
	post := app.call("POST", "/api/v1/system/metrics", admin)
	eq(t, post.status, 405)
	eq(t, post.header("Allow"), "GET,HEAD")
}

func TestMetricsViewNeedsTheSettingsRight(t *testing.T) {
	t.Parallel()
	app := startApp(t, "METRICS_ENABLED", "true")
	admin := app.admin()
	bob := app.user("bob@example.com", false)
	carol := app.user("carol@example.com", false)
	readers := idOf(createGroup(t, app, admin, "Readers"))
	ops := idOf(createGroup(t, app, admin, "Ops"))
	eq(t, app.call("PUT", "/api/v1/groups/"+readers+"/members/"+bob.ID.String(), admin).status, 204)
	eq(t, app.call("PUT", "/api/v1/groups/"+ops+"/members/"+carol.ID.String(), admin).status, 204)
	eq(t, app.send("PUT", "/api/v1/groups/"+readers+"/permissions", admin, obj{"permissions": obj{"scans": []string{"read"}}}).status, 200)
	eq(t, app.send("PUT", "/api/v1/groups/"+ops+"/permissions", admin, obj{"permissions": obj{"clusters": []string{"read"}}}).status, 200)
	bobSession := app.session("bob@example.com", password)
	carolSession := app.session("carol@example.com", password)

	eq(t, app.get("/api/v1/system/metrics", bobSession).status, 200)
	r := app.get("/api/v1/system/metrics", carolSession)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	eq(t, app.get("/api/v1/system/metrics", "").status, 401)

	read := app.pat(admin, "read")
	r = app.bearer("GET", "/api/v1/system/metrics", read, nil)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	eq(t, app.bearer("GET", "/api/v1/system/metrics", app.pat(admin, "settings:read"), nil).status, 200)
	eq(t, app.bearer("GET", "/api/v1/system/metrics/family?name=svcr_http_requests_total", app.pat(bobSession, "settings:read"), nil).status, 200)
}
