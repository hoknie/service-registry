package outbound

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/http/httpproxy"
)

type Options struct {
	CAPEM      []byte
	Timeout    time.Duration
	HTTPSProxy string
	HTTPProxy  string
	NoProxy    string
}

func New(o Options) *http.Client {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if len(o.CAPEM) > 0 {
		roots.AppendCertsFromPEM(o.CAPEM)
	}
	proxy := (&httpproxy.Config{
		HTTPSProxy: o.HTTPSProxy,
		HTTPProxy:  o.HTTPProxy,
		NoProxy:    o.NoProxy,
	}).ProxyFunc()
	timeout := o.Timeout
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
