package linkcheck

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/config"
)

type anyIP struct{}

func (anyIP) Allows(string, netip.Addr) bool { return true }

func newChecker(timeout time.Duration) *CheckerClient {
	return NewWithPolicy(config.OutboundConfig{}, timeout, anyIP{})
}

func code(r linkstypes.Result) int {
	if r.HTTPStatus == nil {
		return 0
	}
	return *r.HTTPStatus
}

func TestStatusesOfResponses(t *testing.T) {
	var gets atomic.Int32
	mux := http.NewServeMux()
	for path, status := range map[string]int{"/ok": 200, "/no-content": 204, "/unauthorized": 401, "/forbidden": 403,
		"/missing": 404, "/gone": 410, "/broken": 500, "/unavailable": 503, "/teapot": 418} {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
	}
	mux.HandleFunc("/head-not-allowed", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		gets.Add(1)
		if r.Header.Get("Range") != "bytes=0-0" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/login-redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login?next=/d/x", http.StatusFound)
	})
	mux.HandleFunc("/sso-redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/SSO/start", http.StatusFound)
	})
	mux.HandleFunc("/other-host", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://accounts.example/", http.StatusFound)
	})
	mux.HandleFunc("/hop/1", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/hop/2", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/hop/2", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/hop/3", http.StatusFound) })
	mux.HandleFunc("/hop/3", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/ok", http.StatusFound) })
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/loop", http.StatusFound) })
	mux.HandleFunc("/cookie", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "s", Value: "1"})
		http.Redirect(w, r, "/cookie", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tests := []struct {
		path   string
		status linkstypes.Status
		code   int
	}{
		{"/ok", linkstypes.StatusOK, 200},
		{"/no-content", linkstypes.StatusOK, 204},
		{"/unauthorized", linkstypes.StatusAuthRequired, 401},
		{"/forbidden", linkstypes.StatusAuthRequired, 403},
		{"/missing", linkstypes.StatusNotFound, 404},
		{"/gone", linkstypes.StatusNotFound, 410},
		{"/broken", linkstypes.StatusServerError, 500},
		{"/unavailable", linkstypes.StatusServerError, 503},
		{"/teapot", linkstypes.StatusUnexpected, 418},
		{"/head-not-allowed", linkstypes.StatusOK, 200},
		{"/login-redirect", linkstypes.StatusAuthRequired, 302},
		{"/sso-redirect", linkstypes.StatusAuthRequired, 302},
		{"/other-host", linkstypes.StatusAuthRequired, 302},
		{"/hop/1", linkstypes.StatusOK, 200},
		{"/loop", linkstypes.StatusRedirect, 302},
		{"/cookie", linkstypes.StatusRedirect, 302},
	}
	c := newChecker(5 * time.Second)
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			r := c.Check(context.Background(), srv.URL+tt.path)
			if r.Status != tt.status || code(r) != tt.code {
				t.Fatalf("got %s %d, want %s %d", r.Status, code(r), tt.status, tt.code)
			}
		})
	}
	if gets.Load() != 1 {
		t.Fatalf("GET after HEAD 405: %d", gets.Load())
	}
}

func TestFailuresOfConnections(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer slow.Close()
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer tlsSrv.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := "http://" + ln.Addr().String() + "/"
	ln.Close()

	c := newChecker(300 * time.Millisecond)
	for name, tt := range map[string]struct {
		url    string
		status linkstypes.Status
	}{
		"timeout":     {slow.URL, linkstypes.StatusTimeout},
		"untrusted":   {tlsSrv.URL, linkstypes.StatusTLSError},
		"refused":     {closed, linkstypes.StatusUnreachable},
		"no such dns": {"http://no-such-host.invalid/", linkstypes.StatusDNSError},
	} {
		t.Run(name, func(t *testing.T) {
			r := c.Check(context.Background(), tt.url)
			if r.Status != tt.status || r.HTTPStatus != nil {
				t.Fatalf("got %s %v", r.Status, code(r))
			}
		})
	}
}

func TestPolicyIsAppliedAtConnectionTime(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer srv.Close()
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	c := NewWithPolicy(config.OutboundConfig{}, 2*time.Second, linkstypes.Policy{AllowPrivate: true})
	for _, u := range []string{
		"http://localhost:" + port + "/",
		"http://127.0.0.1:" + port + "/",
		"http://169.254.169.254/latest/meta-data/",
	} {
		if r := c.Check(context.Background(), u); r.Status != linkstypes.StatusBlocked || r.HTTPStatus != nil {
			t.Errorf("%s: %s", u, r.Status)
		}
	}
	if hits.Load() != 0 {
		t.Fatal("a blocked address was requested")
	}
}

func TestProxyVariablesAreIgnored(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	if r := newChecker(2*time.Second).Check(context.Background(), srv.URL); r.Status != linkstypes.StatusOK {
		t.Fatalf("%s", r.Status)
	}
}

func TestAtMostTwoChecksOfAHostAtATime(t *testing.T) {
	var now, peak atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := now.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
		now.Add(-1)
	}))
	defer srv.Close()
	c := newChecker(5 * time.Second)
	done := make(chan struct{})
	for range 6 {
		go func() {
			c.Check(context.Background(), srv.URL)
			done <- struct{}{}
		}()
	}
	for range 6 {
		<-done
	}
	if peak.Load() > 2 {
		t.Fatalf("peak %d", peak.Load())
	}
}
