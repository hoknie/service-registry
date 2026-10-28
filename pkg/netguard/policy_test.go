package netguard

import (
	"net/netip"
	"testing"
)

func TestAddressPolicy(t *testing.T) {
	open := Policy{AllowPrivate: true, DenyHosts: []string{"*.evil.example", "bad.example"}}
	closed := Policy{AllowPrivate: false, AllowHosts: []string{"*.corp.example", "grafana.internal"}}
	nets := Policy{AllowNets: []netip.Prefix{netip.MustParsePrefix("10.1.0.0/16"), netip.MustParsePrefix("127.0.0.1/32")}, DenyHosts: []string{"bad.example"}}
	tests := []struct {
		name   string
		policy Policy
		host   string
		ip     string
		want   bool
	}{
		{"public", open, "grafana.example", "93.184.216.34", true},
		{"cloud metadata", open, "169.254.169.254", "169.254.169.254", false},
		{"loopback", open, "evil.example", "127.0.0.1", false},
		{"loopback v6", open, "x", "::1", false},
		{"mapped loopback", open, "x", "::ffff:127.0.0.1", false},
		{"this network", open, "x", "0.1.2.3", false},
		{"unspecified", open, "x", "0.0.0.0", false},
		{"link-local v6", open, "x", "fe80::1", false},
		{"multicast", open, "x", "224.0.0.1", false},
		{"broadcast", open, "x", "255.255.255.255", false},
		{"private allowed", open, "jira.internal", "10.0.0.5", true},
		{"cgnat allowed", open, "x", "100.64.1.1", true},
		{"ula allowed", open, "x", "fd00::1", true},
		{"deny list", open, "a.evil.example", "93.184.216.34", false},
		{"deny list exact", open, "BAD.example.", "93.184.216.34", false},
		{"deny list not parent", open, "evil.example", "93.184.216.34", true},
		{"private refused", closed, "jira.internal", "10.0.0.5", false},
		{"cgnat refused", closed, "x", "100.64.1.1", false},
		{"ula refused", closed, "x", "fc00::1", false},
		{"allow list wildcard", closed, "grafana.corp.example", "10.0.0.5", true},
		{"allow list exact", closed, "grafana.internal", "192.168.1.1", true},
		{"allow list is not loopback", closed, "grafana.internal", "127.0.0.1", false},
		{"public when closed", closed, "x", "8.8.8.8", true},
		{"allowed network", nets, "mcp.internal", "10.1.2.3", true},
		{"other private network", nets, "mcp.internal", "10.2.0.1", false},
		{"allowed loopback", nets, "localhost", "127.0.0.1", true},
		{"allowed mapped loopback", nets, "x", "::ffff:127.0.0.1", true},
		{"other loopback", nets, "x", "127.0.0.2", false},
		{"deny list wins over networks", nets, "bad.example", "10.1.2.3", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.policy.Allows(tt.host, netip.MustParseAddr(tt.ip)); got != tt.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestMatchHost(t *testing.T) {
	entries := []string{"app.example", "*.trusted.example"}
	for host, want := range map[string]bool{
		"app.example":          true,
		"APP.example.":         true,
		"a.trusted.example":    true,
		"trusted.example":      false,
		"other.example":        false,
		"app.example.evil.com": false,
	} {
		if got := MatchHost(entries, host); got != want {
			t.Errorf("%s: got %v", host, got)
		}
	}
}
