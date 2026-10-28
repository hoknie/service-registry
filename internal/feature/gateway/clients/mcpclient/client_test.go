package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	gatewayports "svc-registry/internal/feature/gateway/ports"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/pkg/netguard"
	"svc-registry/pkg/testing/mcpfake"
)

var loopback = netguard.Policy{AllowNets: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")}}

func target(url string) gatewayports.Target {
	return gatewayports.Target{ServerID: uuid.New(), Version: "v1", URL: url, Transport: gatewaytypes.TransportStreamable,
		AuthHeader: "Authorization", AuthValue: "Bearer s3cret", Proxy: gatewaytypes.Proxy{Mode: gatewaytypes.ProxyDirect},
		Timeout: 5 * time.Second, MaxBytes: 1 << 20}
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var f *gatewaytypes.Failure
	if !errors.As(err, &f) {
		t.Fatalf("not a failure: %v", err)
	}
	return f.Code
}

func TestCheckReadsInfoAndToolsOverBothTransports(t *testing.T) {
	t.Parallel()
	s := mcpfake.Start(t, mcpfake.Tool{Name: "search", Description: "Search", ReadOnly: true}, mcpfake.Tool{Name: "create_issue"})
	s.RequireHeader("Authorization", "Bearer s3cret")
	s.SetInstructions("Use search first")
	c := New(Options{Policy: loopback, UserAgent: "svc-registry/test", Version: "test"})
	defer c.Close()
	for _, tr := range []struct{ kind, url string }{{gatewaytypes.TransportStreamable, s.URL}, {gatewaytypes.TransportSSE, s.SSE}} {
		tg := target(tr.url)
		tg.Transport = tr.kind
		res, err := c.Check(context.Background(), tg)
		if err != nil {
			t.Fatalf("%s: %v", tr.kind, err)
		}
		if res.Info.Name != "fake" || res.Info.Version != "1.2.3" || res.Info.Instructions != "Use search first" || len(res.Tools) != 2 ||
			res.Tools[1].Name != "search" || !strings.Contains(string(res.Tools[1].Annotations), "readOnlyHint") || res.Tools[0].OutputSchema != nil {
			t.Fatalf("%s: %+v", tr.kind, res)
		}
	}
}

func TestCheckTruncatesTools(t *testing.T) {
	t.Parallel()
	s := mcpfake.Start(t, mcpfake.Tool{Name: "a"}, mcpfake.Tool{Name: "b"}, mcpfake.Tool{Name: "c"})
	c := New(Options{Policy: loopback, MaxTools: 2})
	res, err := c.Check(context.Background(), target(s.URL))
	if err != nil || len(res.Tools) != 2 || !res.Truncated {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestFailuresGetGatewayCodes(t *testing.T) {
	t.Parallel()
	s := mcpfake.Start(t, mcpfake.Tool{Name: "search"})
	s.RequireHeader("Authorization", "Bearer other")
	open := New(Options{Policy: loopback})
	if _, err := open.Check(context.Background(), target(s.URL)); codeOf(t, err) != gatewaytypes.FailUnauthorized {
		t.Fatal(err)
	}
	closed := New(Options{})
	if _, err := closed.Check(context.Background(), target(s.URL)); codeOf(t, err) != gatewaytypes.FailBlocked {
		t.Fatal(err)
	}
	s.RequireHeader("Authorization", "Bearer s3cret")
	s.SetDelay(2 * time.Second)
	tg := target(s.URL)
	tg.Timeout = 300 * time.Millisecond
	if _, err := open.Check(context.Background(), tg); codeOf(t, err) != gatewaytypes.FailTimeout {
		t.Fatal(err)
	}
	if _, err := open.Check(context.Background(), target("http://127.0.0.1:1/mcp")); codeOf(t, err) != gatewaytypes.FailUnreachable {
		t.Fatal(err)
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	t.Parallel()
	hit := false
	dest := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer dest.Close()
	src := httptest.NewServer(http.RedirectHandler(dest.URL, http.StatusTemporaryRedirect))
	defer src.Close()
	c := New(Options{Policy: loopback})
	if _, err := c.Check(context.Background(), target(src.URL)); codeOf(t, err) != gatewaytypes.FailProtocol || hit {
		t.Fatalf("%v, hit %v", err, hit)
	}
}

func TestCallsReuseTheSessionAndRetryOnceWhenItIsGone(t *testing.T) {
	t.Parallel()
	s := mcpfake.Start(t, mcpfake.Tool{Name: "search"})
	s.RequireHeader("Authorization", "Bearer s3cret")
	c := New(Options{Policy: loopback})
	defer c.Close()
	tg := target(s.URL)
	for range 2 {
		res, err := c.Call(context.Background(), tg, "search", json.RawMessage(`{"q":"bug"}`))
		if err != nil || res.IsError || !strings.Contains(string(res.Raw), `search:{\"q\":\"bug\"}`) {
			t.Fatalf("%+v %v", res, err)
		}
	}
	if n := s.Initializations(); n != 1 {
		t.Fatalf("initializations %d", n)
	}
	s.DropSessions()
	if _, err := c.Call(context.Background(), tg, "search", nil); err != nil {
		t.Fatal(err)
	}
	if calls := s.Calls(); len(calls) != 3 || calls[0].Auth != "Bearer s3cret" {
		t.Fatalf("calls %+v", calls)
	}
}

func TestLargeAnswersAreRefused(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("x", 4096)
	s := mcpfake.Start(t, mcpfake.Tool{Name: "dump", Answer: func(json.RawMessage) *sdk.CallToolResult {
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: big}}}
	}})
	c := New(Options{Policy: loopback})
	defer c.Close()
	tg := target(s.URL)
	tg.MaxBytes = 1024
	if _, err := c.Call(context.Background(), tg, "dump", nil); codeOf(t, err) != gatewaytypes.FailTooLarge {
		t.Fatal(err)
	}
}

func TestToolErrorsArePassedThrough(t *testing.T) {
	t.Parallel()
	s := mcpfake.Start(t, mcpfake.Tool{Name: "fail", Answer: func(json.RawMessage) *sdk.CallToolResult {
		return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: "no such issue"}}}
	}})
	c := New(Options{Policy: loopback})
	defer c.Close()
	res, err := c.Call(context.Background(), target(s.URL), "fail", nil)
	if err != nil || !res.IsError || !strings.Contains(string(res.Raw), "no such issue") {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := c.Call(context.Background(), target(s.URL), "missing", nil); codeOf(t, err) != gatewaytypes.FailProtocol {
		t.Fatal(err)
	}
}

func TestCustomProxyCarriesItsCredentials(t *testing.T) {
	t.Parallel()
	seen := make(chan string, 4)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Get("Proxy-Authorization") + " " + r.URL.Host
		w.WriteHeader(http.StatusProxyAuthRequired)
	}))
	defer proxy.Close()
	c := New(Options{Policy: loopback})
	tg := target("http://mcp.example.test/mcp")
	tg.Proxy = gatewaytypes.Proxy{Mode: gatewaytypes.ProxyCustom, URL: &proxy.URL}
	tg.ProxyAuth = "svc:pw"
	if _, err := c.Check(context.Background(), tg); codeOf(t, err) != gatewaytypes.FailProxy {
		t.Fatal(err)
	}
	if got := <-seen; got != "Basic c3ZjOnB3 mcp.example.test" {
		t.Fatal(got)
	}
}
