package config

import "strings"

type MCPOAuthConfig struct {
	Mode                   string   `env:"MCP_OAUTH" envDefault:"off"`
	AccessTTLSecs          uint32   `env:"MCP_OAUTH_ACCESS_TTL_SECS" envDefault:"3600" validate:"min=300,max=86400"`
	RefreshDays            uint32   `env:"MCP_OAUTH_REFRESH_DAYS" envDefault:"30" validate:"min=1,max=365"`
	ClientMetadataMode     string   `env:"MCP_OAUTH_CLIENT_METADATA" envDefault:"on"`
	ClientMetadataHostsRaw string   `env:"MCP_OAUTH_CLIENT_METADATA_HOSTS"`
	ClientMetadataUseProxy bool     `env:"MCP_OAUTH_CLIENT_METADATA_USE_PROXY" envDefault:"false"`
	Enabled                bool     `env:"-"`
	ClientMetadata         bool     `env:"-"`
	ClientMetadataHosts    []string `env:"-"`
}

func (c *MCPOAuthConfig) check() Errors {
	var errs Errors
	var ok bool
	if c.Enabled, ok = onOff(&c.Mode); !ok {
		errs = append(errs, &Error{Var: "MCP_OAUTH", Reason: `must be "on" or "off"`})
	}
	if c.ClientMetadata, ok = onOff(&c.ClientMetadataMode); !ok {
		errs = append(errs, &Error{Var: "MCP_OAUTH_CLIENT_METADATA", Reason: `must be "on" or "off"`})
	}
	if c.ClientMetadataHosts, ok = hostPatterns(c.ClientMetadataHostsRaw); !ok {
		errs = append(errs, &Error{Var: "MCP_OAUTH_CLIENT_METADATA_HOSTS", Reason: "must be host names or *.domain separated by commas"})
	}
	return errs
}

func (c MCPOAuthConfig) ClientDocuments() bool {
	return c.ClientMetadata && len(c.ClientMetadataHosts) > 0
}

func onOff(mode *string) (bool, bool) {
	switch *mode = strings.TrimSpace(*mode); *mode {
	case "on":
		return true, true
	case "off":
		return false, true
	}
	return false, false
}
