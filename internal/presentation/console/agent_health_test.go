package console

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type probes struct{ live, ready bool }

func (p probes) Live() bool  { return p.live }
func (p probes) Ready() bool { return p.ready }

func TestAgentHealthHandler(t *testing.T) {
	t.Parallel()
	h := agentHealthHandler(probes{live: true})
	for _, c := range []struct {
		method, path string
		status       int
		body         string
	}{
		{"GET", "/healthz", 200, `{"status":"ok"}` + "\n"},
		{"HEAD", "/healthz", 200, ""},
		{"GET", "/readyz", 503, `{"status":"unavailable"}` + "\n"},
		{"GET", "/metrics", 404, ""},
		{"POST", "/healthz", 405, ""},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != c.status {
			t.Errorf("%s %s: status %d, want %d", c.method, c.path, rec.Code, c.status)
		}
		if c.body != "" && rec.Body.String() != c.body {
			t.Errorf("%s %s: body %q", c.method, c.path, rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	agentHealthHandler(probes{}).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("a stalled loop: %d", rec.Code)
	}
}
