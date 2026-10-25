package tests

import (
	"fmt"
	"testing"
)

func TestHTTPRequestsAreCountedByRouteTemplate(t *testing.T) {
	t.Parallel()
	app := startApp(t, "METRICS_ENABLED", "true")
	admin := app.admin()
	org := app.nodeID(admin, "organization", "", "acme")
	eq(t, app.get(nodePath(org)+"/activity", admin).status, 200)
	app.call("GET", "/ru/catalog?node="+org, "")
	app.call("GET", "/api/nowhere", "")

	body := app.metrics("")
	eq(t, series(t, body, `svcr_http_requests_total{method="GET",route="/api/v1/catalog/nodes/:id/activity",status="200"}`), 1.0)
	eq(t, series(t, body, `svcr_http_requests_total{method="GET",route="unmatched",status="404"}`), 1.0)
	contains(t, body, `route="web"`)
	lacks(t, body, "/ru/catalog")
	lacks(t, body, org)
	lacks(t, body, "root@example.com")
}

func TestIngestOutcomesAreCounted(t *testing.T) {
	t.Parallel()
	app := startApp(t, "METRICS_ENABLED", "true")
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")
	e := deployed("run-1", "api", "production", "1.0.0", t2)
	eq(t, app.ingest(pid, key, e).status, 201)
	eq(t, app.ingest(pid, key, e).status, 200)
	eq(t, app.ingest(pid, "svcr_wrong", e).status, 401)
	custom := deployed("run-2", "api", "production", "1.0.1", t3)
	custom["type"] = "custom.whatever"
	eq(t, app.ingest(pid, key, custom).status, 422)

	body := app.metrics("")
	eq(t, series(t, body, `svcr_ingest_events_total{code="",outcome="accepted",type="service.deployed"}`), 1.0)
	eq(t, series(t, body, `svcr_ingest_events_total{code="",outcome="replayed",type="service.deployed"}`), 1.0)
	eq(t, series(t, body, `svcr_ingest_events_total{code="auth.invalid_project_key",outcome="rejected",type="service.deployed"}`), 1.0)
	eq(t, series(t, body, `svcr_ingest_events_total{code="ingest.unknown_event_type",outcome="rejected",type="unknown"}`), 1.0)
	eq(t, series(t, body, `svcr_deployments_total{source="event"}`), 1.0)
	lacks(t, body, "custom.whatever")
	lacks(t, body, key)
}

func TestThrottledLoginsAreCounted(t *testing.T) {
	t.Parallel()
	app := startApp(t, "METRICS_ENABLED", "true", "LOGIN_MAX_FAILURES", "1")
	app.user("eve@example.com", false)
	app.session("eve@example.com", password)
	eq(t, app.login("eve@example.com", "wrong-password").status, 401)
	eq(t, app.login("eve@example.com", "wrong-password").status, 429)

	body := app.metrics("")
	eq(t, series(t, body, `svcr_logins_total{method="password",outcome="succeeded"}`), 1.0)
	eq(t, series(t, body, `svcr_logins_total{method="password",outcome="failed"}`), 1.0)
	eq(t, series(t, body, `svcr_logins_total{method="password",outcome="throttled"}`), 1.0)
	lacks(t, body, "eve@example.com")
}

func TestMCPCallsAreCountedPerProject(t *testing.T) {
	t.Parallel()
	d, token := mcpDocs(t, "METRICS_ENABLED", "true")
	hidden := d.app.nodeID(d.admin, "organization", "", "hidden")
	d.app.nodeID(d.admin, "project", hidden, "secret")
	s := d.app.mcpSession(token)
	if _, failed := call(t, s, "list_docs", map[string]any{"project": "acme/api"}); failed {
		t.Fatal("list_docs failed")
	}
	if _, failed := call(t, s, "get_project", map[string]any{"project": "hidden/secret"}); !failed {
		t.Fatal("a hidden project was readable")
	}
	call(t, s, "search_docs", map[string]any{"query": "idempotency"})

	body := d.app.metrics("")
	eq(t, series(t, body, `svcr_mcp_calls_total{kind="tool",name="list_docs",outcome="ok"}`), 1.0)
	eq(t, series(t, body, `svcr_mcp_calls_total{kind="tool",name="get_project",outcome="error"}`), 1.0)
	eq(t, series(t, body, fmt.Sprintf(`svcr_mcp_project_calls_total{organization_id="%s",project_id="%s"}`, d.org, d.id)), 1.0)
	eq(t, series(t, body, `svcr_knowledge_searches_total{mode="text",outcome="ok",source="mcp"}`), 1.0)
	lacks(t, body, hidden)
	lacks(t, body, token)
}
