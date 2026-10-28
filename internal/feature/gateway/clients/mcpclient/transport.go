package mcpclient

import (
	"errors"
	"io"
	"net/http"
	"strings"

	gatewayports "svc-registry/internal/feature/gateway/ports"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/pkg/netguard"
)

func (c *Client) base(t gatewayports.Target) http.RoundTripper {
	p := t.Proxy
	key := p.Mode
	if p.Mode == gatewaytypes.ProxyCustom {
		key += "|" + deref(p.URL) + "|" + deref(p.NoProxy) + "|" + t.ProxyAuth
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if rt, ok := c.transports[key]; ok {
		return rt
	}
	var rt http.RoundTripper
	switch p.Mode {
	case gatewaytypes.ProxyEnvironment:
		rt = netguard.NewProxyTransport(c.opts.Environment, c.opts.Policy, c.opts.Resolver)
	case gatewaytypes.ProxyCustom:
		o := c.opts.Outbound
		o.HTTPSProxy, o.HTTPProxy, o.NoProxy, o.ProxyAuth = deref(p.URL), deref(p.URL), deref(p.NoProxy), t.ProxyAuth
		rt = netguard.NewProxyTransport(o, c.opts.Policy, c.opts.Resolver)
	default:
		rt = netguard.NewTransport(c.opts.Outbound, c.opts.Policy, c.opts.Resolver)
	}
	if c.wrap != nil {
		rt = c.wrap(rt)
	}
	c.transports[key] = rt
	return rt
}

func (c *Client) httpClient(t gatewayports.Target) *http.Client {
	return &http.Client{
		Timeout:   t.Timeout,
		Transport: &guarded{base: c.base(t), header: t.AuthHeader, value: t.AuthValue, agent: c.opts.UserAgent, max: t.MaxBytes},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

type guarded struct {
	base   http.RoundTripper
	header string
	value  string
	agent  string
	max    int64
}

func (g *guarded) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if g.header != "" {
		r.Header.Set(g.header, g.value)
	}
	r.Header.Set("User-Agent", g.agent)
	resp, err := g.base.RoundTrip(r)
	if err != nil {
		return nil, classify(err)
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		_ = resp.Body.Close()
		return nil, gatewaytypes.Fail(gatewaytypes.FailUnauthorized, "the server answered "+resp.Status)
	case resp.StatusCode == http.StatusProxyAuthRequired:
		_ = resp.Body.Close()
		return nil, gatewaytypes.Fail(gatewaytypes.FailProxy, "the proxy answered "+resp.Status)
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		_ = resp.Body.Close()
		return nil, gatewaytypes.Fail(gatewaytypes.FailProtocol, "redirects are not followed: "+resp.Status)
	}
	if r.Method == http.MethodPost && g.max > 0 {
		resp.Body = &limited{ReadCloser: resp.Body, left: g.max + 64<<10}
	}
	return resp, nil
}

type limited struct {
	io.ReadCloser
	left int64
}

func (l *limited) Read(p []byte) (int, error) {
	if l.left <= 0 {
		return 0, gatewaytypes.Fail(gatewaytypes.FailTooLarge, "the answer is larger than MCP_GATEWAY_MAX_RESULT_BYTES")
	}
	if int64(len(p)) > l.left {
		p = p[:l.left]
	}
	n, err := l.ReadCloser.Read(p)
	l.left -= int64(n)
	return n, err
}

func classify(err error) error {
	var f *gatewaytypes.Failure
	switch {
	case errors.As(err, &f):
		return err
	case errors.Is(err, netguard.ErrBlocked):
		return gatewaytypes.Fail(gatewaytypes.FailBlocked, err.Error())
	case strings.Contains(err.Error(), "proxyconnect"):
		return gatewaytypes.Fail(gatewaytypes.FailProxy, err.Error())
	}
	return err
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
