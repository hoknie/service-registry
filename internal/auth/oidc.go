package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"svc-registry/internal/access"
	"svc-registry/internal/config"
)

type OIDC struct {
	client    *http.Client
	publicURL string
	mu        sync.Mutex
	providers map[string]*oidcProvider
}

type oidcProvider struct {
	cfg      config.OAuthProvider
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
}

func NewOIDC(cfg config.OAuthConfig, publicURL string, client *http.Client) *OIDC {
	o := &OIDC{client: client, publicURL: strings.TrimSuffix(publicURL, "/"), providers: map[string]*oidcProvider{}}
	for _, p := range cfg.Providers {
		o.providers[p.Key] = &oidcProvider{cfg: p}
	}
	return o
}

func (o *OIDC) CallbackURL(provider string) string {
	return o.publicURL + "/api/v1/auth/oauth/" + provider + "/callback"
}

func (o *OIDC) context(ctx context.Context) context.Context {
	ctx = oidc.ClientContext(ctx, o.client)
	return context.WithValue(ctx, oauth2.HTTPClient, o.client)
}

func (o *OIDC) ready(ctx context.Context, key string) (*oidcProvider, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	p, ok := o.providers[key]
	if !ok {
		return nil, access.RefuseFailed
	}
	if p.provider != nil {
		return p, nil
	}
	provider, err := oidc.NewProvider(o.context(context.WithoutCancel(ctx)), p.cfg.Issuer)
	if err != nil {
		slog.Warn("oauth provider discovery failed", "provider", key, "error", err)
		return nil, access.RefuseUnavailable
	}
	secret, err := p.cfg.ResolveSecret()
	if err != nil {
		slog.Warn("oauth client secret cannot be read", "provider", key, "error", err)
		return nil, access.RefuseUnavailable
	}
	p.provider = provider
	p.verifier = provider.Verifier(&oidc.Config{ClientID: p.cfg.ClientID})
	p.oauth = &oauth2.Config{ClientID: p.cfg.ClientID, ClientSecret: secret, Endpoint: provider.Endpoint(),
		RedirectURL: o.CallbackURL(key), Scopes: p.cfg.Scopes}
	return p, nil
}

func (o *OIDC) AuthURL(ctx context.Context, provider, state, nonce, verifier string) (string, error) {
	p, err := o.ready(ctx, provider)
	if err != nil {
		return "", err
	}
	return p.oauth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), nil
}

type idClaims struct {
	Subject           string `json:"sub"`
	Email             string `json:"email"`
	EmailVerified     any    `json:"email_verified"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
}

func (o *OIDC) Exchange(ctx context.Context, provider, code, verifier, nonce string) (access.Claims, error) {
	p, err := o.ready(ctx, provider)
	if err != nil {
		return access.Claims{}, err
	}
	ctx = o.context(ctx)
	token, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return access.Claims{}, refused(provider, "code exchange", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return access.Claims{}, refused(provider, "id_token", fmt.Errorf("no id_token in the token response"))
	}
	idToken, err := p.verifier.Verify(ctx, raw)
	if err != nil {
		return access.Claims{}, refused(provider, "id_token", err)
	}
	if idToken.Nonce != nonce {
		return access.Claims{}, refused(provider, "id_token", fmt.Errorf("nonce mismatch"))
	}
	var c idClaims
	all := map[string]any{}
	if err := idToken.Claims(&c); err != nil {
		return access.Claims{}, refused(provider, "id_token claims", err)
	}
	_ = idToken.Claims(&all)
	out := access.Claims{Subject: idToken.Subject, Email: c.Email, EmailVerified: truthy(c.EmailVerified), Name: c.Name,
		PreferredUsername: c.PreferredUsername}
	groups, hasGroups := stringList(all[p.cfg.GroupsClaim])
	if out.Email == "" || !hasGroups {
		info, err := p.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
		if err == nil && info.Subject == out.Subject {
			var u idClaims
			more := map[string]any{}
			_ = info.Claims(&u)
			_ = info.Claims(&more)
			if out.Email == "" {
				out.Email, out.EmailVerified = info.Email, info.EmailVerified || truthy(u.EmailVerified)
			}
			if !hasGroups {
				groups, _ = stringList(more[p.cfg.GroupsClaim])
			}
			if out.Name == "" {
				out.Name = u.Name
			}
		}
	}
	out.Groups = groups
	return out, nil
}

func refused(provider, what string, err error) error {
	slog.Info("oauth login refused", "provider", provider, "step", what, "error", err)
	return access.RefuseFailed
}

func truthy(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return strings.EqualFold(b, "true")
	}
	return false
}

func stringList(v any) ([]string, bool) {
	switch l := v.(type) {
	case []any:
		out := make([]string, 0, len(l))
		for _, e := range l {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out, true
	case string:
		return []string{l}, true
	}
	return nil, false
}

func RandomString() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
