package outbound

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/http/httpproxy"

	"svc-registry/internal/config"
)

func New(cfg config.OutboundConfig) *http.Client {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if len(cfg.CAPEM) > 0 {
		roots.AppendCertsFromPEM(cfg.CAPEM)
	}
	proxy := (&httpproxy.Config{
		HTTPSProxy: cfg.HTTPSProxy,
		HTTPProxy:  cfg.HTTPProxy,
		NoProxy:    cfg.NoProxy,
	}).ProxyFunc()
	timeout := time.Duration(cfg.TimeoutSecs) * time.Second
	transport := &http.Transport{
		Proxy:                 func(r *http.Request) (*url.URL, error) { return proxy(r.URL) },
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:       &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   4,
		ForceAttemptHTTP2:     true,
	}
	return &http.Client{Transport: transport, Timeout: timeout}
}
