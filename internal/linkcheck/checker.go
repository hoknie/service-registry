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

	"svc-registry/internal/config"
	"svc-registry/internal/links"
)

const (
	maxRedirects = 3
	perHost      = 2
	userAgent    = "svc-registry-link-check"
)

var loginPaths = []string{"/login", "/signin", "/oauth", "/sso", "/auth"}

type Checker struct {
	client  *http.Client
	timeout time.Duration
	mu      sync.Mutex
	hosts   map[string]chan struct{}
}

func New(out config.OutboundConfig, lc config.LinkCheckConfig) *Checker {
	policy := links.Policy{AllowPrivate: lc.AllowPrivate, AllowHosts: lc.AllowHosts, DenyHosts: lc.DenyHosts}
	return NewWithPolicy(out, time.Duration(lc.TimeoutSecs)*time.Second, policy)
}

func NewWithPolicy(out config.OutboundConfig, timeout time.Duration, policy Policy) *Checker {
	return &Checker{
		client: &http.Client{
			Transport:     newTransport(out, policy, net.DefaultResolver),
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		timeout: timeout,
		hosts:   map[string]chan struct{}{},
	}
}

func (c *Checker) Check(ctx context.Context, raw string) links.Result {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	res := c.check(ctx, raw)
	res.DurationMS = int(time.Since(start) / time.Millisecond)
	return res
}

func (c *Checker) check(ctx context.Context, raw string) links.Result {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return links.Result{Status: links.StatusUnexpected}
	}
	release, err := c.acquire(ctx, strings.ToLower(u.Hostname()))
	if err != nil {
		return links.Result{Status: classify(err)}
	}
	defer release()
	var last *int
	for hop := 0; ; hop++ {
		code, location, err := c.probe(ctx, u)
		if err != nil {
			return links.Result{Status: classify(err), HTTPStatus: last}
		}
		last = &code
		if code < 300 || code > 399 || location == "" {
			return links.Result{Status: links.StatusOf(code), HTTPStatus: last}
		}
		next, err := u.Parse(location)
		if err != nil {
			return links.Result{Status: links.StatusUnexpected, HTTPStatus: last}
		}
		if !strings.EqualFold(next.Hostname(), u.Hostname()) || isLoginPath(next.Path) {
			return links.Result{Status: links.StatusAuthRequired, HTTPStatus: last}
		}
		if hop == maxRedirects || (next.Scheme != "http" && next.Scheme != "https") {
			return links.Result{Status: links.StatusRedirect, HTTPStatus: last}
		}
		u = next
	}
}

func (c *Checker) probe(ctx context.Context, u *url.URL) (int, string, error) {
	code, location, err := c.send(ctx, http.MethodHead, u)
	if err == nil && (code == http.StatusMethodNotAllowed || code == http.StatusNotImplemented) {
		return c.send(ctx, http.MethodGet, u)
	}
	return code, location, err
}

func (c *Checker) send(ctx context.Context, method string, u *url.URL) (int, string, error) {
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
	resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Location"), nil
}

func (c *Checker) acquire(ctx context.Context, host string) (func(), error) {
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

func classify(err error) links.Status {
	var dnsErr *net.DNSError
	var certErr *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var invalidCert x509.CertificateInvalidError
	var recordErr tls.RecordHeaderError
	var netErr net.Error
	switch {
	case errors.Is(err, errBlocked):
		return links.StatusBlocked
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return links.StatusTimeout
	case errors.As(err, &dnsErr):
		return links.StatusDNSError
	case errors.As(err, &certErr), errors.As(err, &unknownAuthority), errors.As(err, &hostnameErr),
		errors.As(err, &invalidCert), errors.As(err, &recordErr):
		return links.StatusTLSError
	}
	if strings.Contains(err.Error(), "tls:") {
		return links.StatusTLSError
	}
	return links.StatusUnreachable
}
