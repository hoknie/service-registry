package forgeclient

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	domain "svc-registry/internal/feature/forge"
)

func fromStatus(resp *http.Response, ownerCall bool, cause error) error {
	if resp == nil {
		return network(cause)
	}
	switch status := resp.StatusCode; {
	case status == http.StatusUnauthorized:
		return domain.ErrUnauthorized
	case status == http.StatusNotFound && ownerCall:
		return domain.ErrOwnerNotFound
	case status == http.StatusTooManyRequests,
		status == http.StatusForbidden && (resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != ""):
		return &domain.RateLimited{Reset: resetOf(resp.Header)}
	default:
		return &domain.Upstream{Status: status, Detail: detail(cause)}
	}
}

func network(cause error) error {
	if errors.Is(cause, context.Canceled) {
		return domain.ErrInterrupted
	}
	return &domain.Upstream{Detail: detail(cause)}
}

func resetOf(h http.Header) time.Time {
	for _, name := range []string{"X-RateLimit-Reset", "RateLimit-Reset"} {
		if v, err := strconv.ParseInt(h.Get(name), 10, 64); err == nil && v > 0 {
			if v < 1_000_000_000 {
				return time.Now().Add(time.Duration(v) * time.Second)
			}
			return time.Unix(v, 0)
		}
	}
	if v, err := strconv.ParseInt(h.Get("Retry-After"), 10, 64); err == nil && v >= 0 {
		return time.Now().Add(time.Duration(v) * time.Second)
	}
	return time.Time{}
}

func detail(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if i := strings.IndexByte(s, '?'); i >= 0 {
		if j := strings.IndexAny(s[i:], " :"); j >= 0 {
			s = s[:i] + s[i+j:]
		} else {
			s = s[:i]
		}
	}
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

func limitReadme(content []byte) (*string, bool) {
	if len(content) <= domain.ReadmeMaxBytes {
		s := string(content)
		return &s, false
	}
	cut := domain.ReadmeMaxBytes
	for cut > 0 && content[cut]&0xC0 == 0x80 {
		cut--
	}
	s := string(content[:cut])
	return &s, true
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
