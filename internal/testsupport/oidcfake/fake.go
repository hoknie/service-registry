package oidcfake

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

type Person struct {
	Subject          string
	Email            string
	EmailVerified    bool
	Name             string
	Groups           []string
	GroupsInUserinfo bool
	Extra            map[string]any
}

type grant struct {
	person    Person
	nonce     string
	challenge string
	clientID  string
}

type Provider struct {
	srv           *httptest.Server
	key           *rsa.PrivateKey
	ClientID      string
	Secret        string
	mu            sync.Mutex
	person        Person
	grants        map[string]grant
	tokens        map[string]Person
	deny          bool
	wrongAudience bool
	forger        *rsa.PrivateKey
}

func Start(t testing.TB) *Provider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{key: key, ClientID: "registry", Secret: "client-secret", grants: map[string]grant{}, tokens: map[string]Person{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("/jwks", p.jwks)
	mux.HandleFunc("/authorize", p.authorize)
	mux.HandleFunc("/token", p.token)
	mux.HandleFunc("/userinfo", p.userinfo)
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

func (p *Provider) Issuer() string { return p.srv.URL }

func (p *Provider) SignIn(person Person) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.person = person
}

func (p *Provider) Deny(deny bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deny = deny
}

func (p *Provider) WrongAudience(wrong bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.wrongAudience = wrong
}

func (p *Provider) Forge(forge bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.forger = nil
	if forge {
		p.forger, _ = rsa.GenerateKey(rand.Reader, 2048)
	}
}

func (p *Provider) Stop() { p.srv.Close() }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (p *Provider) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer": p.Issuer(), "authorization_endpoint": p.Issuer() + "/authorize", "token_endpoint": p.Issuer() + "/token",
		"jwks_uri": p.Issuer() + "/jwks", "userinfo_endpoint": p.Issuer() + "/userinfo",
		"id_token_signing_alg_values_supported": []string{"RS256"}, "response_types_supported": []string{"code"},
		"subject_types_supported": []string{"public"}, "code_challenge_methods_supported": []string{"S256"},
	})
}

func (p *Provider) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &p.key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
}

func (p *Provider) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	redirect, err := url.Parse(q.Get("redirect_uri"))
	if err != nil || q.Get("code_challenge_method") != "S256" || q.Get("response_type") != "code" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	back := redirect.Query()
	back.Set("state", q.Get("state"))
	if p.deny {
		back.Set("error", "access_denied")
	} else {
		code := randomString()
		p.grants[code] = grant{person: p.person, nonce: q.Get("nonce"), challenge: q.Get("code_challenge"), clientID: q.Get("client_id")}
		back.Set("code", code)
	}
	redirect.RawQuery = back.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

func (p *Provider) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	p.mu.Lock()
	g, found := p.grants[r.PostForm.Get("code")]
	delete(p.grants, r.PostForm.Get("code"))
	wrongAud := p.wrongAudience
	p.mu.Unlock()
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !found || id != p.ClientID || secret != p.Secret || g.clientID != p.ClientID ||
		base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "invalid_grant"})
		return
	}
	aud := p.ClientID
	if wrongAud {
		aud = "someone-else"
	}
	claims := map[string]any{"iss": p.Issuer(), "sub": g.person.Subject, "aud": aud, "nonce": g.nonce,
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix()}
	if g.person.Email != "" {
		claims["email"], claims["email_verified"] = g.person.Email, g.person.EmailVerified
	}
	if g.person.Name != "" {
		claims["name"] = g.person.Name
	}
	if g.person.Groups != nil && !g.person.GroupsInUserinfo {
		claims["groups"] = g.person.Groups
	}
	for k, v := range g.person.Extra {
		claims[k] = v
	}
	access := randomString()
	p.mu.Lock()
	p.tokens[access] = g.person
	p.mu.Unlock()
	writeJSON(w, map[string]any{"access_token": access, "token_type": "Bearer", "expires_in": 3600, "id_token": p.sign(claims)})
}

func (p *Provider) userinfo(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	p.mu.Lock()
	person, ok := p.tokens[token[min(len(token), len("Bearer ")):]]
	p.mu.Unlock()
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	out := map[string]any{"sub": person.Subject, "email": person.Email, "email_verified": person.EmailVerified}
	if person.Groups != nil {
		out["groups"] = person.Groups
	}
	writeJSON(w, out)
}

func (p *Provider) sign(claims map[string]any) string {
	p.mu.Lock()
	key := p.key
	if p.forger != nil {
		key = p.forger
	}
	p.mu.Unlock()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: "k1"}}, nil)
	if err != nil {
		panic(err)
	}
	payload, _ := json.Marshal(claims)
	obj, err := signer.Sign(payload)
	if err != nil {
		panic(err)
	}
	s, err := obj.CompactSerialize()
	if err != nil {
		panic(err)
	}
	return s
}

func randomString() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
