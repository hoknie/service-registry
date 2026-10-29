package outbound

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/http/httpproxy"
)

type Options struct {
	CAPEM      []byte
	Timeout    time.Duration
	HTTPSProxy string
	HTTPProxy  string
	NoProxy    string
	ProxyAuth  string
}

func (o Options) ProxyFunc() func(*http.Request) (*url.URL, error) {
	proxy := (&httpproxy.Config{HTTPSProxy: o.HTTPSProxy, HTTPProxy: o.HTTPProxy, NoProxy: o.NoProxy}).ProxyFunc()
	return func(r *http.Request) (*url.URL, error) {
		u, err := proxy(r.URL)
		if u == nil || err != nil || o.ProxyAuth == "" {
			return u, err
		}
		with := *u
		if name, pass, ok := strings.Cut(o.ProxyAuth, ":"); ok {
			with.User = url.UserPassword(name, pass)
		} else {
			with.User = url.User(o.ProxyAuth)
		}
		return &with, nil
	}
}

func (o Options) RootCAs() *x509.CertPool {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if len(o.CAPEM) > 0 {
		roots.AppendCertsFromPEM(o.CAPEM)
	}
	return roots
}

func New(o Options) *http.Client {
	roots := o.RootCAs()
	timeout := o.Timeout
	transport := &http.Transport{
		Proxy:               o.ProxyFunc(),
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:     &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout: 10 * time.Second,
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConnsPerHost: 4,
		ForceAttemptHTTP2:   true,
	}
	return &http.Client{Transport: transport, Timeout: timeout}
}
