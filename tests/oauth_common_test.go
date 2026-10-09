package tests

import (
	"net/url"
	"strings"
	"testing"

	"svc-registry/internal/testsupport/oidcfake"
)

const publicURL = "https://registry.example"

func startOAuthApp(t testing.TB, extra ...string) (*testApp, *oidcfake.Provider) {
	t.Helper()
	idp := oidcfake.Start(t)
	vars := []string{"PUBLIC_URL", publicURL, "OAUTH_PROVIDERS", "corp", "OAUTH_CORP_KIND", "oidc",
		"OAUTH_CORP_ISSUER", idp.Issuer(), "OAUTH_CORP_CLIENT_ID", idp.ClientID, "OAUTH_CORP_CLIENT_SECRET", idp.Secret}
	return startApp(t, append(vars, extra...)...), idp
}

func (a *testApp) oauthLogin(start, cookie string) reply {
	a.t.Helper()
	r := a.call("GET", start, cookie)
	eq(a.t, r.status, 302, "start: "+r.text())
	browser := cookieNamed(a.t, r, "registry_oauth")
	toIdP := r.header("Location")
	resp, err := client.Get(toIdP)
	if err != nil {
		a.t.Fatal(err)
	}
	resp.Body.Close()
	eq(a.t, resp.StatusCode, 302, "authorize")
	back, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		a.t.Fatal(err)
	}
	eq(a.t, strings.HasPrefix(back.String(), publicURL+"/api/v1/auth/oauth/"), true, back.String())
	cookies := browser
	if cookie != "" {
		cookies += "; " + cookie
	}
	return a.call("GET", back.RequestURI(), cookies)
}

func cookieNamed(t testing.TB, r reply, name string) string {
	t.Helper()
	for _, set := range r.headers.Values("Set-Cookie") {
		pair, _, _ := strings.Cut(set, ";")
		if strings.HasPrefix(pair, name+"=") {
			return pair
		}
	}
	t.Fatalf("no %s cookie in %v", name, r.headers.Values("Set-Cookie"))
	return ""
}

func hasCookie(r reply, name string) bool {
	for _, set := range r.headers.Values("Set-Cookie") {
		pair, _, _ := strings.Cut(set, ";")
		if v, ok := strings.CutPrefix(pair, name+"="); ok && v != "" {
			return true
		}
	}
	return false
}

func ann() oidcfake.Person {
	return oidcfake.Person{Subject: "sub-ann", Email: "Ann@Example.com", EmailVerified: true, Name: "Ann"}
}
