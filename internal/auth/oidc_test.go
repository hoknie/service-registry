package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"svc-registry/internal/access"
	"svc-registry/internal/config"
	"svc-registry/internal/testsupport/oidcfake"
)

func login(t *testing.T, o *OIDC, nonce, verifier string) string {
	t.Helper()
	authURL, err := o.AuthURL(context.Background(), "corp", "st", nonce, verifier)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return loc.Query().Get("code")
}

func newOIDC(p *oidcfake.Provider) *OIDC {
	return NewOIDC(config.OAuthConfig{Providers: []config.OAuthProvider{{Key: "corp", Kind: "oidc", Issuer: p.Issuer(),
		ClientID: p.ClientID, ClientSecret: p.Secret, Scopes: []string{"openid", "email"}, GroupsClaim: "groups"}}},
		"https://registry.example", http.DefaultClient)
}

func TestExchangeVerifiesTheIDToken(t *testing.T) {
	p := oidcfake.Start(t)
	o := newOIDC(p)
	p.SignIn(oidcfake.Person{Subject: "u-1", Email: "ann@example.com", EmailVerified: true, Name: "Ann", Groups: []string{"sre"}})
	verifier := "a-verifier-of-at-least-forty-three-characters-xxxxx"
	code := login(t, o, "n-1", verifier)
	c, err := o.Exchange(context.Background(), "corp", code, verifier, "n-1")
	if err != nil || c.Subject != "u-1" || c.Email != "ann@example.com" || !c.EmailVerified || c.Name != "Ann" || len(c.Groups) != 1 {
		t.Fatalf("%+v %v", c, err)
	}
	code = login(t, o, "n-2", verifier)
	if _, err := o.Exchange(context.Background(), "corp", code, verifier, "other"); !errors.Is(err, access.RefuseFailed) {
		t.Fatal("nonce", err)
	}
	code = login(t, o, "n-3", verifier)
	if _, err := o.Exchange(context.Background(), "corp", code, verifier+"x", "n-3"); !errors.Is(err, access.RefuseFailed) {
		t.Fatal("verifier", err)
	}
	p.WrongAudience(true)
	code = login(t, o, "n-4", verifier)
	if _, err := o.Exchange(context.Background(), "corp", code, verifier, "n-4"); !errors.Is(err, access.RefuseFailed) {
		t.Fatal("audience", err)
	}
	p.WrongAudience(false)
	p.Forge(true)
	code = login(t, o, "n-6", verifier)
	if _, err := o.Exchange(context.Background(), "corp", code, verifier, "n-6"); !errors.Is(err, access.RefuseFailed) {
		t.Fatal("signature", err)
	}
	p.Forge(false)
	p.SignIn(oidcfake.Person{Subject: "u-1", Email: "ann@example.com", EmailVerified: true, Extra: map[string]any{"exp": time.Now().Add(-time.Hour).Unix()}})
	code = login(t, o, "n-7", verifier)
	if _, err := o.Exchange(context.Background(), "corp", code, verifier, "n-7"); !errors.Is(err, access.RefuseFailed) {
		t.Fatal("expired", err)
	}
	p.SignIn(oidcfake.Person{Subject: "u-2", Email: "bob@example.com", EmailVerified: true, Groups: []string{"a", "b"}, GroupsInUserinfo: true})
	code = login(t, o, "n-5", verifier)
	if c, err := o.Exchange(context.Background(), "corp", code, verifier, "n-5"); err != nil || len(c.Groups) != 2 {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestUnreachableProvider(t *testing.T) {
	p := oidcfake.Start(t)
	o := newOIDC(p)
	p.Stop()
	if _, err := o.AuthURL(context.Background(), "corp", "s", "n", "v"); !errors.Is(err, access.RefuseUnavailable) {
		t.Fatal(err)
	}
}
