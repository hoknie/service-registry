package outbound

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestServerOfAnUnknownCAIsRejectedUntilTrusted(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	defer srv.Close()
	if _, err := New(Options{Timeout: 5 * time.Second}).Get(srv.URL); err == nil {
		t.Fatal("self-signed certificate accepted")
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	resp, err := New(Options{Timeout: 5 * time.Second, CAPEM: caPEM}).Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatal(resp.StatusCode)
	}
}

func TestProxyFollowsNoProxy(t *testing.T) {
	c := New(Options{Timeout: 5 * time.Second, HTTPSProxy: "http://proxy.example:3128", NoProxy: "internal.example"})
	proxy := c.Transport.(*http.Transport).Proxy
	via := func(raw string) string {
		u, _ := url.Parse(raw)
		p, err := proxy(&http.Request{URL: u})
		if err != nil || p == nil {
			return ""
		}
		return p.Host
	}
	if got := via("https://api.github.com/orgs/x"); got != "proxy.example:3128" {
		t.Errorf("github via %q", got)
	}
	if got := via("https://git.internal.example/api/v1"); got != "" {
		t.Errorf("internal via %q", got)
	}
}
