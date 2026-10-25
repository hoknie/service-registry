package tests

import (
	"strconv"
	"strings"
	"testing"

	"svc-registry/internal/testsupport"
)

const metricsToken = "metrics-token-0123456789"

func (a *testApp) metrics(token string) string {
	a.t.Helper()
	var headers []string
	if token != "" {
		headers = append(headers, "authorization", "Bearer "+token)
	}
	r := request(a.t, a.addr, "GET", "/metrics", "", headers...)
	if r.status != 200 {
		a.t.Fatalf("metrics: %d %s", r.status, r.body)
	}
	return r.text()
}

func series(t testing.TB, body, name string) float64 {
	t.Helper()
	for line := range strings.SplitSeq(body, "\n") {
		if value, ok := strings.CutPrefix(line, name+" "); ok {
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				t.Fatalf("%s: %v", line, err)
			}
			return v
		}
	}
	return 0
}

func TestMetricsAreOffByDefault(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	r := app.call("GET", "/metrics", "")
	eq(t, r.status, 307, r.text())
	eq(t, r.header("Location"), "/en/metrics")
	lacks(t, r.text(), "svcr_build_info")
}

func TestMetricsPageServesTheExposition(t *testing.T) {
	t.Parallel()
	app := startApp(t, "METRICS_ENABLED", "true")
	dist := testsupport.Dist{Dir: app.dist}
	dist.WriteExport(t)
	dist.Write(t, "metrics.html", "<html>not metrics</html>")

	r := app.call("GET", "/metrics", "")
	eq(t, r.status, 200, r.text())
	eq(t, r.header("Content-Type"), "text/plain; version=0.0.4; charset=utf-8; escaping=underscores")
	eq(t, r.header("Cache-Control"), "no-store")
	contains(t, r.text(), `svcr_build_info{go_version="go`)
	contains(t, r.text(), `version="`)
	lacks(t, r.text(), "not metrics")

	head := app.call("HEAD", "/metrics", "")
	eq(t, head.status, 200)
	eq(t, len(head.body), 0)
	post := app.call("POST", "/metrics", "")
	eq(t, post.status, 405)
	eq(t, post.header("Allow"), "GET,HEAD")
	eq(t, app.call("GET", "/metrics/", "").status != 200, true)
}

func TestMetricsTokenIsRequiredWhenSet(t *testing.T) {
	t.Parallel()
	app := startApp(t, "METRICS_ENABLED", "true", "METRICS_TOKEN", metricsToken)
	admin := app.admin()
	for _, headers := range [][]string{nil, {"authorization", "Bearer wrong-token-0123456789"}, {"cookie", admin},
		{"authorization", metricsToken}} {
		r := request(t, app.addr, "GET", "/metrics", "", headers...)
		eq(t, r.status, 401, headers)
		eq(t, r.header("WWW-Authenticate"), "Bearer")
		eq(t, code(t, r), any("auth.invalid_metrics_token"))
	}
	contains(t, app.metrics(metricsToken), "svcr_build_info")
}
