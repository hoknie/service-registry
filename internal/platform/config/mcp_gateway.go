package config

import (
	"fmt"
	"net/netip"
	"strings"
	"time"
)

type MCPGatewayConfig struct {
	Enabled           bool           `env:"MCP_GATEWAY_ENABLED" envDefault:"true"`
	CheckIntervalSecs int64          `env:"MCP_GATEWAY_CHECK_INTERVAL_SECS" envDefault:"300"`
	CallTimeoutSecs   int64          `env:"MCP_GATEWAY_CALL_TIMEOUT_SECS" envDefault:"60"`
	MaxResultBytes    int64          `env:"MCP_GATEWAY_MAX_RESULT_BYTES" envDefault:"1048576"`
	MaxTools          int64          `env:"MCP_GATEWAY_MAX_TOOLS" envDefault:"200"`
	CallLogDays       int64          `env:"MCP_GATEWAY_CALL_LOG_DAYS" envDefault:"30"`
	UseProxy          bool           `env:"MCP_GATEWAY_USE_PROXY" envDefault:"false"`
	AllowHostsRaw     []string       `env:"MCP_GATEWAY_ALLOW_HOSTS" envSeparator:","`
	AllowHosts        []string       `env:"-"`
	AllowNets         []netip.Prefix `env:"-"`
}

func (c MCPGatewayConfig) CheckInterval() time.Duration {
	return time.Duration(c.CheckIntervalSecs) * time.Second
}

func (c MCPGatewayConfig) CallTimeout() time.Duration {
	return time.Duration(c.CallTimeoutSecs) * time.Second
}

func (c MCPGatewayConfig) CallLogRetention() time.Duration {
	return time.Duration(c.CallLogDays) * 24 * time.Hour
}

func (c *MCPGatewayConfig) check() Errors {
	if !c.Enabled {
		return nil
	}
	var errs Errors
	for _, r := range []skillsRange{
		{"MCP_GATEWAY_CHECK_INTERVAL_SECS", c.CheckIntervalSecs, 30, 86400},
		{"MCP_GATEWAY_CALL_TIMEOUT_SECS", c.CallTimeoutSecs, 1, 600},
		{"MCP_GATEWAY_MAX_RESULT_BYTES", c.MaxResultBytes, 1024, 16777216},
		{"MCP_GATEWAY_MAX_TOOLS", c.MaxTools, 1, 1000},
		{"MCP_GATEWAY_CALL_LOG_DAYS", c.CallLogDays, 1, 365},
	} {
		if r.value < r.min || r.value > r.max {
			errs = append(errs, &Error{Var: r.name, Reason: fmt.Sprintf("must be from %d to %d, got %d", r.min, r.max, r.value)})
		}
	}
	return append(errs, c.parseAllowHosts()...)
}

func (c *MCPGatewayConfig) parseAllowHosts() Errors {
	c.AllowHosts, c.AllowNets = nil, nil
	for _, raw := range c.AllowHostsRaw {
		entry := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case entry == "":
		case strings.Contains(entry, "/"):
			p, err := netip.ParsePrefix(entry)
			if err != nil {
				return Errors{{Var: "MCP_GATEWAY_ALLOW_HOSTS", Reason: fmt.Sprintf("%q is not a valid network", entry)}}
			}
			c.AllowNets = append(c.AllowNets, p.Masked())
		default:
			if ip, err := netip.ParseAddr(entry); err == nil {
				c.AllowNets = append(c.AllowNets, netip.PrefixFrom(ip, ip.BitLen()))
				continue
			}
			if strings.ContainsAny(entry, " :@") {
				return Errors{{Var: "MCP_GATEWAY_ALLOW_HOSTS", Reason: fmt.Sprintf("%q is not a host name", entry)}}
			}
			c.AllowHosts = append(c.AllowHosts, entry)
		}
	}
	return nil
}
