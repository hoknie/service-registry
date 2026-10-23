package clientmeta

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/pkg/netguard"
	"svc-registry/pkg/outbound"
)

type anyIP struct{}

func (anyIP) Allows(string, netip.Addr) bool { return true }

func server(t *testing.T, h http.HandlerFunc) (*httptest.Server, outbound.Options) {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	return srv, outbound.Options{CAPEM: ca}
}

func TestFetchReadsTheDocument(t *testing.T) {
	t.Parallel()
	srv, o := server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Cache-Control", "max-age=600")
		_, _ = w.Write([]byte(`{"client_id":"x"}`))
	})
	got, err := NewWithPolicy(o, anyIP{}).Fetch(context.Background(), srv.URL+"/client.json")
	if err != nil || string(got.Body) != `{"client_id":"x"}` || got.CacheControl != "max-age=600" {
		t.Fatalf("%+v %v", got, err)
	}
}

func reasonOfFetch(t *testing.T, err error) string {
	t.Helper()
	var refusal *accesstypes.ClientMetadataError
	if !errors.Is(err, ErrUnavailable) || !errors.As(err, &refusal) || refusal.Code != accesstypes.CodeClientMetadataUnavailable {
		t.Fatalf("got %v", err)
	}
	return refusal.Reason
}

func TestFetchRefusals(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		h      http.HandlerFunc
		reason string
	}{
		"redirect": {func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		}, "redirect"},
		"status":       {func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }, "status_404"},
		"server error": {func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }, "status_502"},
		"too large": {func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("a", 5<<10+1)))
		}, "too_large"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv, o := server(t, tc.h)
			_, err := NewWithPolicy(o, anyIP{}).Fetch(context.Background(), srv.URL+"/client.json")
			if got := reasonOfFetch(t, err); got != tc.reason {
				t.Fatalf("reason %q, want %q", got, tc.reason)
			}
		})
	}
}

func TestFetchRefusalReasonsOfTheConnection(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	slow, o := server(t, func(http.ResponseWriter, *http.Request) { <-release })
	t.Cleanup(func() { close(release) })
	untrusted, _ := server(t, func(http.ResponseWriter, *http.Request) {})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := NewWithPolicy(o, anyIP{}).Fetch(ctx, slow.URL+"/client.json")
	if got := reasonOfFetch(t, err); got != "timeout" {
		t.Fatalf("slow server: %q", got)
	}
	_, err = NewWithPolicy(outbound.Options{}, anyIP{}).Fetch(context.Background(), untrusted.URL+"/client.json")
	if got := reasonOfFetch(t, err); got != "tls" {
		t.Fatalf("untrusted certificate: %q", got)
	}
	_, err = NewWithPolicy(outbound.Options{}, netguard.Policy{}).Fetch(context.Background(), untrusted.URL+"/client.json")
	if got := reasonOfFetch(t, err); got != "blocked_address" {
		t.Fatalf("loopback: %q", got)
	}
	_, err = NewWithPolicy(outbound.Options{}, anyIP{}).Fetch(context.Background(), "https://127.0.0.1:1/client.json")
	if got := reasonOfFetch(t, err); got != "connection" {
		t.Fatalf("closed port: %q", got)
	}
}

func TestReasonOfErrors(t *testing.T) {
	t.Parallel()
	for want, err := range map[string]error{
		"dns":             fmt.Errorf("dial: %w", &net.DNSError{Err: "no such host", Name: "x.invalid", IsNotFound: true}),
		"timeout":         fmt.Errorf("get: %w", context.DeadlineExceeded),
		"blocked_address": fmt.Errorf("dial: %w", netguard.ErrBlocked),
		"tls":             fmt.Errorf("handshake: %w", x509.UnknownAuthorityError{}),
		"connection":      errors.New("connection reset by peer"),
	} {
		if got := reasonOf(err); got != want {
			t.Fatalf("%v: %q, want %q", err, got, want)
		}
	}
}

func TestFetchRefusesLoopbackByDefault(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	srv, o := server(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })
	_, err := NewWithPolicy(o, netguard.Policy{}).Fetch(context.Background(), srv.URL+"/client.json")
	if !errors.Is(err, ErrUnavailable) || !errors.Is(err, netguard.ErrBlocked) || called.Load() {
		t.Fatalf("got %v, called %v", err, called.Load())
	}
}

func TestProxiedFetchRefusesLiteralLoopbackBeforeTheProxy(t *testing.T) {
	t.Parallel()
	var proxied atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxied.Store(true)
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(proxy.Close)
	_, err := NewProxied(outbound.Options{HTTPSProxy: proxy.URL}).Fetch(context.Background(), "https://127.0.0.1:1/client.json")
	if !errors.Is(err, ErrUnavailable) || !errors.Is(err, netguard.ErrBlocked) || proxied.Load() {
		t.Fatalf("got %v, proxied %v", err, proxied.Load())
	}
}
