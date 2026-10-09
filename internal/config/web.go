package config

import (
	"net/url"
	"strings"
)

type WebConfig struct {
	DistDir   string `env:"WEB_DIST_DIR" envDefault:"web/out"`
	PublicURL string `env:"PUBLIC_URL"`
}

func (c *WebConfig) check() Errors {
	c.DistDir = strings.TrimSpace(c.DistDir)
	c.PublicURL = strings.TrimSpace(c.PublicURL)
	if c.PublicURL == "" {
		return nil
	}
	u, err := url.Parse(c.PublicURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" ||
		u.Fragment != "" || u.User != nil || strings.HasSuffix(c.PublicURL, "/") {
		return Errors{{Var: "PUBLIC_URL", Reason: "must be http(s)://host[:port][/path] without a trailing slash, query or fragment"}}
	}
	return nil
}
