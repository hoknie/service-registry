package netguard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"

	"svc-registry/pkg/outbound"
)

var numericLabel = regexp.MustCompile(`^(0x[0-9a-f]*|[0-9]+)$`)

type proxyTransport struct {
	base   *http.Transport
	policy Policy
}

func NewProxyTransport(o outbound.Options, p Policy, resolver *net.Resolver) http.RoundTripper {
	t := NewTransport(o, p, resolver)
	t.Proxy = o.ProxyFunc()
	guarded := t.DialContext
	proxies := proxyAddrs(o.HTTPSProxy, o.HTTPProxy)
	plain := (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if proxies[strings.ToLower(addr)] {
			return plain(ctx, network, addr)
		}
		return guarded(ctx, network, addr)
	}
	return &proxyTransport{base: t, policy: p}
}

func (t *proxyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	host := strings.TrimSuffix(strings.ToLower(r.URL.Hostname()), ".")
	if t.policy.HostDenied(host) || host == "localhost" || strings.HasSuffix(host, ".localhost") || numericHost(host) {
		return nil, fmt.Errorf("%s: %w", host, ErrBlocked)
	}
	if ip, err := netip.ParseAddr(host); err == nil && !t.policy.Allows(host, ip) {
		return nil, fmt.Errorf("%s: %w", host, ErrBlocked)
	}
	return t.base.RoundTrip(r)
}

func numericHost(host string) bool {
	if _, err := netip.ParseAddr(host); err == nil || host == "" {
		return false
	}
	labels := strings.Split(host, ".")
	if len(labels) > 4 {
		return false
	}
	for _, l := range labels {
		if !numericLabel.MatchString(l) {
			return false
		}
	}
	return true
}

func proxyAddrs(raw ...string) map[string]bool {
	out := map[string]bool{}
	for _, r := range raw {
		if r == "" {
			continue
		}
		if !strings.Contains(r, "://") {
			r = "http://" + r
		}
		u, err := url.Parse(r)
		if err != nil || u.Hostname() == "" {
			continue
		}
		port := u.Port()
		if port == "" {
			port = map[string]string{"https": "443", "socks5": "1080"}[u.Scheme]
		}
		if port == "" {
			port = "80"
		}
		out[strings.ToLower(net.JoinHostPort(u.Hostname(), port))] = true
	}
	return out
}
