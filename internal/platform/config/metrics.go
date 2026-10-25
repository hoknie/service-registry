package config

import (
	"strings"
	"unicode"
)

type MetricsConfig struct {
	Enabled     bool   `env:"METRICS_ENABLED" envDefault:"false"`
	TokenRaw    string `env:"METRICS_TOKEN"`
	RefreshSecs uint32 `env:"METRICS_REFRESH_SECS" envDefault:"60" validate:"min=10,max=3600"`
	Token       string `env:"-"`
}

func (c *MetricsConfig) check() Errors {
	raw := strings.TrimSpace(c.TokenRaw)
	if raw == "" {
		return nil
	}
	token := raw
	if strings.HasPrefix(raw, "env:") || strings.HasPrefix(raw, "file:") {
		v, err := ResolveSecretRef(raw)
		if err != nil {
			return Errors{&Error{Var: "METRICS_TOKEN", Reason: "reference cannot be resolved: " + err.Error()}}
		}
		token = v
	}
	if n := len(token); n < 16 || n > 256 || strings.ContainsFunc(token, unicode.IsSpace) {
		return Errors{&Error{Var: "METRICS_TOKEN", Reason: "must be 16 to 256 characters without spaces"}}
	}
	c.Token = token
	return nil
}
