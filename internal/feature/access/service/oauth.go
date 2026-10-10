package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
)

const LoginStateTTL = 600

type Provider struct {
	Key         string
	DisplayName string
}

func (s *Service) Providers() ([]Provider, string) {
	out := []Provider{}
	for _, p := range s.cfg.OAuth.Providers {
		out = append(out, Provider{Key: p.Key, DisplayName: p.DisplayName})
	}
	return out, s.cfg.OAuth.PasswordLogin
}

func (s *Service) providerConfig(key string) (config.OAuthProvider, bool) {
	for _, p := range s.cfg.OAuth.Providers {
		if p.Key == key {
			return p, true
		}
	}
	return config.OAuthProvider{}, false
}

func SafeNext(next string) *string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") ||
		strings.ContainsAny(next, "\r\n") || len(next) > 2048 {
		return nil
	}
	return &next
}

type LoginStart struct {
	URL       string
	BrowserID string
}

func (s *Service) StartLogin(ctx context.Context, provider, next string, linkUser *uuid.UUID) (LoginStart, error) {
	if _, ok := s.providerConfig(provider); !ok {
		return LoginStart{}, apperr.New(apperr.NotFound)
	}
	var values [4]string
	for i := range values {
		v, err := auth.RandomString()
		if err != nil {
			return LoginStart{}, apperr.Internalf("no randomness: %v", err)
		}
		values[i] = v
	}
	browser, st, nonce, verifier := values[0], values[1], values[2], values[3]
	url, err := s.oauth.AuthURL(ctx, provider, st, nonce, verifier)
	if err != nil {
		return LoginStart{}, &apperr.Error{Kind: apperr.Unavailable, What: "login provider", Code: string(access.RefuseUnavailable)}
	}
	err = s.loginStates.Insert(ctx, access.NewLoginState{ID: uuid.Must(uuid.NewV7()), BrowserHash: sha256.Sum256([]byte(browser)),
		Provider: provider, State: st, Nonce: nonce, Verifier: verifier, Next: SafeNext(next), LinkUserID: linkUser, TTLSecs: LoginStateTTL})
	if err != nil {
		return LoginStart{}, apperr.Wrap(err)
	}
	return LoginStart{URL: url, BrowserID: browser}, nil
}

type LoginCallback struct {
	Provider  string
	BrowserID string
	State     string
	Code      string
	Error     string
}

type LoginDone struct {
	Token string
	Next  string
	Link  bool
}

func (s *Service) CompleteLogin(ctx context.Context, cb LoginCallback, previousToken *string) (LoginDone, error) {
	cfg, ok := s.providerConfig(cb.Provider)
	if !ok || cb.State == "" {
		return LoginDone{}, access.RefuseState
	}
	ls, err := s.loginStates.Take(ctx, sha256.Sum256([]byte(cb.BrowserID)), cb.State, cb.Provider)
	if err != nil {
		return LoginDone{}, apperr.Wrap(err)
	}
	if ls == nil {
		return LoginDone{}, access.RefuseState
	}
	done := LoginDone{Next: "/", Link: ls.LinkUserID != nil}
	if ls.Next != nil {
		done.Next = *ls.Next
	}
	if done.Link {
		done.Next = "/account?tab=login"
	}
	if cb.Error != "" || cb.Code == "" {
		return done, access.RefuseDenied
	}
	claims, err := s.oauth.Exchange(ctx, cb.Provider, cb.Code, ls.Verifier, ls.Nonce)
	if err != nil {
		return done, err
	}
	var email *string
	if claims.Email != "" {
		e := strings.ToLower(strings.TrimSpace(claims.Email))
		email = &e
	}
	identity, user, err := s.identities.Find(ctx, cb.Provider, claims.Subject)
	if err != nil {
		return done, apperr.Wrap(err)
	}
	if done.Link {
		return done, s.linkIdentity(ctx, *ls.LinkUserID, identity, cb.Provider, claims.Subject, email)
	}
	var byEmail *access.User
	if identity == nil && email != nil {
		found, err := s.users.FindCredentialsByEmail(ctx, *email)
		if err != nil {
			return done, apperr.Wrap(err)
		}
		if found != nil {
			byEmail = &found.User
		}
	}
	policy := access.ProviderPolicy{Key: cfg.Key, AutoProvision: cfg.AutoProvision, AllowedDomains: cfg.AllowedDomains}
	outcome, err := access.Decide(user, byEmail, claims, policy)
	if err != nil {
		return done, err
	}
	newIdentity := access.NewIdentity{ID: uuid.Must(uuid.NewV7()), Provider: cb.Provider, Subject: claims.Subject, Email: email}
	switch outcome {
	case access.SignIn:
		if err := s.identities.Touch(ctx, identity.ID, email); err != nil {
			return done, apperr.Wrap(err)
		}
	case access.LinkAndSignIn:
		user = byEmail
		newIdentity.UserID = user.ID
		if _, err := s.identities.Insert(ctx, newIdentity); err != nil {
			return done, apperr.Wrap(err)
		}
	case access.Provision:
		addr, err := access.ValidateEmail(*email)
		if err != nil {
			return done, access.RefuseNotProvisioned
		}
		created, err := s.identities.CreateUser(ctx, access.NewUser{ID: uuid.Must(uuid.NewV7()), Email: addr,
			DisplayName: access.ProvisionedName(claims)}, newIdentity)
		if err != nil {
			return done, apperr.Wrap(err)
		}
		user = &created
	}
	groups := access.GroupsFor(claims.Groups, groupRules(cfg))
	if err := s.groups.SyncManaged(ctx, user.ID, access.OAuthSource(cb.Provider), groups); err != nil {
		return done, apperr.Wrap(err)
	}
	signedIn, err := s.openSession(ctx, *user, access.OAuthMethod(cb.Provider), previousToken)
	if err != nil {
		return done, err
	}
	done.Token = signedIn.Token
	return done, nil
}

func (s *Service) linkIdentity(ctx context.Context, userID uuid.UUID, existing *access.Identity, provider, subject string, email *string) error {
	if existing != nil {
		if existing.UserID != userID {
			return access.RefuseIdentityTaken
		}
		return nil
	}
	_, err := s.identities.Insert(ctx, access.NewIdentity{ID: uuid.Must(uuid.NewV7()), UserID: userID, Provider: provider,
		Subject: subject, Email: email})
	var refusal access.Refusal
	if errors.As(err, &refusal) {
		return refusal
	}
	return apperr.Wrap(err)
}

func groupRules(cfg config.OAuthProvider) []access.GroupRule {
	out := make([]access.GroupRule, 0, len(cfg.GroupMap))
	for _, r := range cfg.GroupMap {
		out = append(out, access.GroupRule{Value: r.Value, Group: r.Group})
	}
	return out
}

func (s *Service) PruneLoginStates(ctx context.Context) (int64, error) {
	n, err := s.loginStates.Prune(ctx)
	return n, apperr.Wrap(err)
}
