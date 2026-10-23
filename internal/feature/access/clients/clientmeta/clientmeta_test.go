package clientmeta

import (
	"context"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"

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

func TestFetchRefusals(t *testing.T) {
	t.Parallel()
	tests := map[string]http.HandlerFunc{
		"redirect": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		},
		"status": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) },
		"too large": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("a", 5<<10+1)))
		},
	}
	for name, h := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv, o := server(t, h)
			if _, err := NewWithPolicy(o, anyIP{}).Fetch(context.Background(), srv.URL+"/client.json"); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("got %v", err)
			}
		})
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
