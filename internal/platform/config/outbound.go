package config

import (
	"crypto/x509"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type OutboundConfig struct {
	CAFile      string `env:"HTTP_CA_FILE"`
	CAPEM       []byte `env:"-"`
	TimeoutSecs uint64 `env:"HTTP_CLIENT_TIMEOUT_SECS" envDefault:"30" validate:"min=1,max=600"`

	HTTPSProxy      string `env:"HTTPS_PROXY"`
	HTTPSProxyLower string `env:"https_proxy"`
	HTTPProxy       string `env:"HTTP_PROXY"`
	HTTPProxyLower  string `env:"http_proxy"`
	NoProxy         string `env:"NO_PROXY"`
	NoProxyLower    string `env:"no_proxy"`
}

func (c *OutboundConfig) check() Errors {
	var errs Errors
	c.CAFile = strings.TrimSpace(c.CAFile)
	if c.CAFile != "" {
		pem, err := os.ReadFile(c.CAFile)
		switch {
		case err != nil:
			errs = append(errs, &Error{Var: "HTTP_CA_FILE", Reason: fmt.Sprintf("cannot be read: %v", err)})
		case !x509.NewCertPool().AppendCertsFromPEM(pem):
			errs = append(errs, &Error{Var: "HTTP_CA_FILE", Reason: "contains no PEM certificate"})
		default:
			c.CAPEM = pem
		}
	}
	c.HTTPSProxy = firstSet(c.HTTPSProxy, c.HTTPSProxyLower)
	c.HTTPProxy = firstSet(c.HTTPProxy, c.HTTPProxyLower)
	c.NoProxy = firstSet(c.NoProxy, c.NoProxyLower)
	for name, v := range map[string]string{"HTTPS_PROXY": c.HTTPSProxy, "HTTP_PROXY": c.HTTPProxy} {
		if v == "" {
			continue
		}
		if u, err := url.Parse(v); err != nil || u.Host == "" {
			errs = append(errs, &Error{Var: name, Reason: "must be a proxy URL such as http://proxy:3128"})
		}
	}
	return errs
}

func firstSet(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}
