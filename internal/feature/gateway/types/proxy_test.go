package types

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func ptr(s string) *string { return &s }

func TestProxyFieldsFollowTheMode(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   Proxy
		ok   bool
	}{
		{"direct", Proxy{Mode: ProxyDirect}, true},
		{"environment", Proxy{Mode: ProxyEnvironment}, true},
		{"proxy", Proxy{Mode: ProxyCustom, URL: ptr("http://proxy.acme.internal:3128"), NoProxy: ptr("internal.example")}, true},
		{"credentials in url", Proxy{Mode: ProxyCustom, URL: ptr("http://user:pass@proxy:3128")}, false},
		{"no port", Proxy{Mode: ProxyCustom, URL: ptr("http://proxy")}, false},
		{"socks", Proxy{Mode: ProxyCustom, URL: ptr("socks5://proxy:1080")}, false},
		{"url without proxy mode", Proxy{Mode: ProxyDirect, URL: ptr("http://proxy:3128")}, false},
		{"unknown mode", Proxy{Mode: "tunnel"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := CleanProxy(c.in)
			if (err == nil) != c.ok {
				t.Fatalf("err %v", err)
			}
		})
	}
}

func TestNearestProxyWins(t *testing.T) {
	t.Parallel()
	org := ProxyOwner{Kind: OwnerOrganization, ID: uuid.New(), Name: "acme"}
	server := ProxyOwner{Kind: OwnerServer, ID: uuid.New(), Name: "wiki"}
	inherited := []ProxyLink{{Owner: server}, {Owner: ProxyOwner{Kind: OwnerFolder}}, {Owner: org, Proxy: &Proxy{Mode: ProxyCustom, URL: ptr("http://proxy.acme.internal:3128")}}}
	p, from := NearestProxy(inherited, false)
	if p.Mode != ProxyCustom || from == nil || from.ID != org.ID {
		t.Fatalf("%+v %+v", p, from)
	}
	inherited[0].Proxy = &Proxy{Mode: ProxyDirect}
	if p, from := NearestProxy(inherited, false); p.Mode != ProxyDirect || from.ID != server.ID {
		t.Fatalf("%+v %+v", p, from)
	}
	if p, from := NearestProxy(nil, true); p.Mode != ProxyEnvironment || from != nil {
		t.Fatalf("%+v %+v", p, from)
	}
	if p, _ := NearestProxy(nil, false); p.Mode != ProxyDirect {
		t.Fatalf("%+v", p)
	}
}

func TestFailuresKeepTheirCodeAndHideSecrets(t *testing.T) {
	t.Parallel()
	if got := FailureCode(Fail(FailUnauthorized, "401")); got != FailUnauthorized {
		t.Fatal(got)
	}
	if got := FailureCode(errors.New("dial tcp: refused")); got != FailUnreachable {
		t.Fatal(got)
	}
	if got := Redact("bad token s3cr3t-value in header", "s3cr3t-value"); got != "bad token *** in header" {
		t.Fatal(got)
	}
	rec := CheckOf(uuid.New(), nil, Fail(FailUnauthorized, "token s3cr3t-value refused"), 12, "s3cr3t-value")
	if rec.Outcome != OutcomeFailed || *rec.Code != FailUnauthorized || *rec.Detail != "token *** refused" {
		t.Fatalf("%+v", rec)
	}
}
