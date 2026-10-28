package config_test

import (
	"net/netip"
	"reflect"
	"testing"

	"svc-registry/internal/platform/config"
)

func TestMCPGatewayDefaults(t *testing.T) {
	t.Parallel()
	c, err := config.Load[config.MCPGatewayConfig](lookup())
	if err != nil {
		t.Fatal(err)
	}
	if !c.Enabled || c.CheckIntervalSecs != 300 || c.CallTimeoutSecs != 60 || c.MaxResultBytes != 1<<20 || c.MaxTools != 200 ||
		c.CallLogDays != 30 || c.UseProxy || len(c.AllowHosts) != 0 || len(c.AllowNets) != 0 {
		t.Fatalf("defaults %+v", c)
	}
}

func TestMCPGatewayVariablesAreChecked(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		env  []string
		want []string
	}{
		{[]string{"MCP_GATEWAY_CALL_TIMEOUT_SECS", "0"}, []string{"MCP_GATEWAY_CALL_TIMEOUT_SECS"}},
		{[]string{"MCP_GATEWAY_CHECK_INTERVAL_SECS", "10", "MCP_GATEWAY_MAX_TOOLS", "1001", "MCP_GATEWAY_CALL_LOG_DAYS", "366",
			"MCP_GATEWAY_MAX_RESULT_BYTES", "100"},
			[]string{"MCP_GATEWAY_CALL_LOG_DAYS", "MCP_GATEWAY_CHECK_INTERVAL_SECS", "MCP_GATEWAY_MAX_RESULT_BYTES", "MCP_GATEWAY_MAX_TOOLS"}},
		{[]string{"MCP_GATEWAY_ALLOW_HOSTS", "10.0.0.0/33"}, []string{"MCP_GATEWAY_ALLOW_HOSTS"}},
		{[]string{"MCP_GATEWAY_ALLOW_HOSTS", "http://x"}, []string{"MCP_GATEWAY_ALLOW_HOSTS"}},
		{[]string{"MCP_GATEWAY_ENABLED", "maybe"}, []string{"MCP_GATEWAY_ENABLED"}},
	} {
		_, err := config.Load[config.MCPGatewayConfig](lookup(c.env...))
		if got := vars(t, err); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%v: %v", c.env, got)
		}
	}
}

func TestMCPGatewayAllowHostsSplitIntoHostsAndNetworks(t *testing.T) {
	t.Parallel()
	c, err := config.Load[config.MCPGatewayConfig](lookup("MCP_GATEWAY_ALLOW_HOSTS", " 10.0.0.0/8, mcp.internal ,*.corp.example,127.0.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("127.0.0.1/32")}
	if !reflect.DeepEqual(c.AllowNets, want) || !reflect.DeepEqual(c.AllowHosts, []string{"mcp.internal", "*.corp.example"}) {
		t.Fatalf("hosts %v nets %v", c.AllowHosts, c.AllowNets)
	}
}

func TestDisabledMCPGatewaySkipsTheChecks(t *testing.T) {
	t.Parallel()
	c, err := config.Load[config.MCPGatewayConfig](lookup("MCP_GATEWAY_ENABLED", "false", "MCP_GATEWAY_CALL_TIMEOUT_SECS", "0"))
	if err != nil || c.Enabled {
		t.Fatalf("config %+v, %v", c, err)
	}
}
