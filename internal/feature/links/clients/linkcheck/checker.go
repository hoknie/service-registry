package linkcheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/netguard"
)

const (
	maxRedirects = 3
	perHost      = 2
	userAgent    = "svc-registry-link-check"
)

var loginPaths = []string{"/login", "/signin", "/oauth", "/sso", "/auth"}

type CheckerClient struct {
	client  *http.Client
	timeout time.Duration
	mu      sync.Mutex
	hosts   map[string]chan struct{}
}

func New(out config.OutboundConfig, lc config.LinkCheckConfig) *CheckerClient {
	return NewWithPolicy(out, time.Duration(lc.TimeoutSecs)*time.Second, lc.Policy())
}

func NewWithPolicy(out config.OutboundConfig, timeout time.Duration, policy netguard.Allower) *CheckerClient {
	return &CheckerClient{
		client: &http.Client{
			Transport:     netguard.NewTransport(out.Options(), policy, net.DefaultResolver),
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		timeout: timeout,
		hosts:   map[string]chan struct{}{},
	}
}

func (c *CheckerClient) Check(ctx context.Context, raw string) linkstypes.Result {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	res := c.check(ctx, raw)
	res.DurationMS = int(time.Since(start) / time.Millisecond)
	return res
}

func (c *CheckerClient) check(ctx context.Context, raw string) linkstypes.Result {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return linkstypes.Result{Status: linkstypes.StatusUnexpected}
	}
	release, err := c.acquire(ctx, strings.ToLower(u.Hostname()))
	if err != nil {
		return linkstypes.Result{Status: classify(err)}
	}
	defer release()
	var last *int
	for hop := 0; ; hop++ {
		code, location, err := c.probe(ctx, u)
		if err != nil {
			return linkstypes.Result{Status: classify(err), HTTPStatus: last}
		}
		last = &code
		if code < 300 || code > 399 || location == "" {
			return linkstypes.Result{Status: linkstypes.StatusOf(code), HTTPStatus: last}
		}
		next, err := u.Parse(location)
		if err != nil {
			return linkstypes.Result{Status: linkstypes.StatusUnexpected, HTTPStatus: last}
		}
		if !strings.EqualFold(next.Hostname(), u.Hostname()) || isLoginPath(next.Path) {
			return linkstypes.Result{Status: linkstypes.StatusAuthRequired, HTTPStatus: last}
		}
		if hop == maxRedirects || (next.Scheme != "http" && next.Scheme != "https") {
			return linkstypes.Result{Status: linkstypes.StatusRedirect, HTTPStatus: last}
		}
		u = next
	}
}

func (c *CheckerClient) probe(ctx context.Context, u *url.URL) (int, string, error) {
	code, location, err := c.send(ctx, http.MethodHead, u)
	if err == nil && (code == http.StatusMethodNotAllowed || code == http.StatusNotImplemented) {
		return c.send(ctx, http.MethodGet, u)
	}
	return code, location, err
}

func (c *CheckerClient) send(ctx context.Context, method string, u *url.URL) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("User-Agent", userAgent)
	if method == http.MethodGet {
		req.Header.Set("Range", "bytes=0-0")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Location"), nil
}

func (c *CheckerClient) acquire(ctx context.Context, host string) (func(), error) {
	c.mu.Lock()
	slots, ok := c.hosts[host]
	if !ok {
		slots = make(chan struct{}, perHost)
		c.hosts[host] = slots
	}
	c.mu.Unlock()
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func isLoginPath(path string) bool {
	path = strings.ToLower(path)
	for _, p := range loginPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func classify(err error) linkstypes.Status {
	if errors.Is(err, netguard.ErrBlocked) {
		return linkstypes.StatusBlocked
	}
	if netErr, ok := errors.AsType[net.Error](err); errors.Is(err, context.DeadlineExceeded) || ok && netErr.Timeout() {
		return linkstypes.StatusTimeout
	}
	if _, ok := errors.AsType[*net.DNSError](err); ok {
		return linkstypes.StatusDNSError
	}
	if tlsError(err) || strings.Contains(err.Error(), "tls:") {
		return linkstypes.StatusTLSError
	}
	return linkstypes.StatusUnreachable
}

func tlsError(err error) bool {
	if _, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		return true
	}
	if _, ok := errors.AsType[x509.UnknownAuthorityError](err); ok {
		return true
	}
	if _, ok := errors.AsType[x509.HostnameError](err); ok {
		return true
	}
	if _, ok := errors.AsType[x509.CertificateInvalidError](err); ok {
		return true
	}
	_, ok := errors.AsType[tls.RecordHeaderError](err)
	return ok
}

func (c *CheckerClient) Instrument(wrap func(http.RoundTripper) http.RoundTripper) {
	c.client.Transport = wrap(c.client.Transport)
}
