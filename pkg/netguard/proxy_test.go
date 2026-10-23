package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"svc-registry/pkg/outbound"
)

func TestProxyTransportReachesNamesThroughTheProxy(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	var seen atomic.Value
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		seen.Store(r.URL.String())
		w.WriteHeader(http.StatusNoContent)
	}))
	defer proxy.Close()
	policy := Policy{AllowPrivate: false, DenyHosts: []string{"bad.example"}}
	client := &http.Client{Transport: NewProxyTransport(outbound.Options{HTTPProxy: proxy.URL, HTTPSProxy: proxy.URL}, policy,
		net.DefaultResolver), Timeout: 5 * time.Second}
	resp, err := client.Post("http://receiver.example/hook", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent || seen.Load() != "http://receiver.example/hook" {
		t.Fatalf("status %d, proxy saw %v", resp.StatusCode, seen.Load())
	}
	for _, raw := range []string{"http://127.0.0.1:8080/hook", "http://[::1]/x", "http://169.254.169.254/latest/",
		"http://10.0.0.5/x", "http://localhost/x", "http://api.localhost/x", "http://2130706433/x", "http://0x7f.1/x",
		"http://bad.example/x"} {
		before := hits.Load()
		_, err := client.Post(raw, "application/json", nil)
		if !errors.Is(err, ErrBlocked) {
			t.Errorf("%s: %v", raw, err)
		}
		if hits.Load() != before {
			t.Errorf("%s reached the proxy", raw)
		}
	}
}

func TestProxyTransportChecksConnectionsPastTheProxy(t *testing.T) {
	t.Parallel()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	defer proxy.Close()
	rt := NewProxyTransport(outbound.Options{HTTPSProxy: proxy.URL, NoProxy: "localhost.direct"}, Policy{AllowPrivate: true},
		net.DefaultResolver).(*proxyTransport)
	ctx := context.Background()
	conn, err := rt.base.DialContext(ctx, "tcp", proxy.Listener.Addr().String())
	if err != nil {
		t.Fatalf("proxy dial: %v", err)
	}
	_ = conn.Close()
	if _, err := rt.base.DialContext(ctx, "tcp", "localhost:80"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("direct loopback dial: %v", err)
	}
}

func TestProxyAddressesGetDefaultPorts(t *testing.T) {
	t.Parallel()
	got := proxyAddrs("proxy.example:3128", "https://Secure.example", "http://plain.example", "")
	for _, want := range []string{"proxy.example:3128", "secure.example:443", "plain.example:80"} {
		if !got[want] {
			t.Errorf("no %s in %v", want, got)
		}
	}
	if numericHost("cafe.deadbeef") || numericHost("api.example") || !numericHost("127.1") {
		t.Error("numeric host classification")
	}
}
