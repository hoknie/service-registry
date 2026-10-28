package hostingclient

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	reposynctypes "svc-registry/internal/feature/reposync/types"
)

func fromStatus(resp *http.Response, ownerCall bool, cause error) error {
	if resp == nil {
		return network(cause)
	}
	switch status := resp.StatusCode; {
	case status == http.StatusUnauthorized:
		return reposynctypes.ErrUnauthorized
	case status == http.StatusNotFound && ownerCall:
		return reposynctypes.ErrOwnerNotFound
	case status == http.StatusTooManyRequests,
		status == http.StatusForbidden && (resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != ""):
		return &reposynctypes.RateLimited{Reset: resetOf(resp.Header)}
	default:
		return &reposynctypes.Upstream{Status: status, Detail: detail(cause)}
	}
}

func network(cause error) error {
	if errors.Is(cause, context.Canceled) {
		return reposynctypes.ErrInterrupted
	}
	return &reposynctypes.Upstream{Detail: detail(cause)}
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
	if len(content) <= reposynctypes.ReadmeMaxBytes {
		s := string(content)
		return &s, false
	}
	cut := reposynctypes.ReadmeMaxBytes
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
