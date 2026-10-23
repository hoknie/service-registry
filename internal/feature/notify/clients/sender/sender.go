package sender

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/pkg/netguard"
	"svc-registry/pkg/outbound"
)

const (
	perHost      = 2
	maxReadBytes = 4 << 10
)

type SenderClient struct {
	client  *http.Client
	timeout time.Duration
	mu      sync.Mutex
	hosts   map[string]chan struct{}
}

func New(o outbound.Options, timeout time.Duration, allow netguard.Allower) *SenderClient {
	return newClient(netguard.NewTransport(o, allow, net.DefaultResolver), timeout)
}

func NewProxied(o outbound.Options, timeout time.Duration, p netguard.Policy) *SenderClient {
	return newClient(netguard.NewProxyTransport(o, p, net.DefaultResolver), timeout)
}

func newClient(rt http.RoundTripper, timeout time.Duration) *SenderClient {
	return &SenderClient{
		client: &http.Client{
			Transport:     rt,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		timeout: timeout,
		hosts:   map[string]chan struct{}{},
	}
}

func (c *SenderClient) Send(ctx context.Context, r notifytypes.Request) notifytypes.Response {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	out := c.send(ctx, r)
	out.DurationMS = int(time.Since(start) / time.Millisecond)
	return out
}

func (c *SenderClient) send(ctx context.Context, r notifytypes.Request) notifytypes.Response {
	u, err := url.Parse(r.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return failure(notifytypes.CodeURLUnavailable, "the address is not an http(s) URL", false)
	}
	release, err := c.acquire(ctx, strings.ToLower(u.Hostname()))
	if err != nil {
		return classify(err)
	}
	defer release()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(r.Body))
	if err != nil {
		return failure(notifytypes.CodeURLUnavailable, "the request cannot be built", false)
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return classify(err)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxReadBytes))
	_ = resp.Body.Close()
	status := resp.StatusCode
	out := notifytypes.Response{HTTPStatus: &status}
	switch {
	case status >= 200 && status < 300:
		return out
	case status >= 300 && status < 400:
		out.ErrorCode, out.ErrorMessage = new(notifytypes.CodeRedirect), new("the receiver answered with a redirect")
	default:
		out.ErrorCode, out.ErrorMessage = new(notifytypes.CodeHTTPStatus), new("the receiver answered HTTP "+strconv.Itoa(status))
		out.Retryable = notifytypes.Retryable(status)
		out.RetryAfter = retryAfter(resp.Header.Get("Retry-After"), status)
	}
	return out
}

func retryAfter(raw string, status int) time.Duration {
	if raw == "" || (status != http.StatusTooManyRequests && status != http.StatusServiceUnavailable) {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if at, err := http.ParseTime(raw); err == nil {
		return max(time.Until(at), 0)
	}
	return 0
}

func (c *SenderClient) acquire(ctx context.Context, host string) (func(), error) {
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

func failure(code, message string, retryable bool) notifytypes.Response {
	return notifytypes.Response{ErrorCode: &code, ErrorMessage: &message, Retryable: retryable}
}

func classify(err error) notifytypes.Response {
	if errors.Is(err, netguard.ErrBlocked) {
		return failure(notifytypes.CodeBlocked, "the address is forbidden by the notification address policy", false)
	}
	if netErr, ok := errors.AsType[net.Error](err); errors.Is(err, context.DeadlineExceeded) || ok && netErr.Timeout() {
		return failure(notifytypes.CodeTimeout, "the receiver did not answer in time", true)
	}
	if _, ok := errors.AsType[*net.DNSError](err); ok {
		return failure(notifytypes.CodeDNSError, "the receiver's host name does not resolve", true)
	}
	if tlsError(err) || strings.Contains(err.Error(), "tls:") {
		return failure(notifytypes.CodeTLSError, "the TLS connection to the receiver failed", true)
	}
	return failure(notifytypes.CodeUnreachable, "the receiver is unreachable", true)
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
