package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"svc-registry/pkg/testing/mcpfake"
)

func TestGatewayCallsAndChecksAreCounted(t *testing.T) {
	t.Parallel()
	a := startGateway(t, "METRICS_ENABLED", "true", "MCP_GATEWAY_CALL_TIMEOUT_SECS", "1")
	admin := a.admin()
	fake := mcpfake.Start(t, mcpfake.Tool{Name: "search"})
	jira := idOf(a.liveMCPServer(admin, fake, obj{"slug": "jira"}))
	s := a.mcpSession(a.pat(admin, "mcp"))
	call(t, s, "jira__search", map[string]any{"q": "x"})
	fake.SetDelay(2 * time.Second)
	if _, failed := call(t, s, "jira__search", nil); !failed {
		t.Fatal("the slow call did not fail")
	}
	a.services.Sampler.SampleNow(context.Background())
	body := a.metrics("")
	eq(t, series(t, body, fmt.Sprintf(`svcr_mcp_gateway_calls_total{outcome="ok",server_id="%s"}`, jira)), 1.0)
	eq(t, series(t, body, fmt.Sprintf(`svcr_mcp_gateway_calls_total{outcome="failed",server_id="%s"}`, jira)), 1.0)
	eq(t, series(t, body, `svcr_mcp_calls_total{kind="tool",name="gateway",outcome="error"}`), 1.0)
	eq(t, series(t, body, `svcr_mcp_gateway_checks_total{outcome="ok"}`), 1.0)
	eq(t, series(t, body, `svcr_mcp_gateway_servers{state="ok"}`), 1.0)
	eq(t, series(t, body, fmt.Sprintf(`svcr_mcp_gateway_server_info{server_id="%s",slug="jira"}`, jira)), 1.0)
	eq(t, series(t, body, `svcr_outbound_requests_total{result="2xx",target="mcp_gateway"}`) > 0, true)
	lacks(t, body, fake.URL)
}
