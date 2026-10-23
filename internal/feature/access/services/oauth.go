package services

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
)

const LoginStateTTL = 600

type Provider struct {
	Key         string
	DisplayName string
}

func (s *OAuthService) Providers() ([]Provider, string) {
	out := []Provider{}
	for _, p := range s.cfg.OAuth.Providers {
		out = append(out, Provider{Key: p.Key, DisplayName: p.DisplayName})
	}
	return out, s.cfg.OAuth.PasswordLogin
}

func (s *OAuthService) providerConfig(key string) (config.OAuthProvider, bool) {
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

func (s *OAuthService) StartLogin(ctx context.Context, provider, next string, linkUser *uuid.UUID) (LoginStart, error) {
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
		return LoginStart{}, &apperr.Error{Kind: apperr.Unavailable, What: "login provider", Code: string(accesstypes.RefuseUnavailable)}
	}
	err = s.loginStateRepository.Insert(ctx, accesstypes.NewLoginState{ID: uuid.Must(uuid.NewV7()), BrowserHash: sha256.Sum256([]byte(browser)),
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

func (s *OAuthService) CompleteLogin(ctx context.Context, cb LoginCallback, previousToken *string) (LoginDone, error) {
	cfg, ok := s.providerConfig(cb.Provider)
	if !ok || cb.State == "" {
		return LoginDone{}, accesstypes.RefuseState
	}
	ls, err := s.loginStateRepository.Take(ctx, sha256.Sum256([]byte(cb.BrowserID)), cb.State, cb.Provider)
	if err != nil {
		return LoginDone{}, apperr.Wrap(err)
	}
	if ls == nil {
		return LoginDone{}, accesstypes.RefuseState
	}
	done := LoginDone{Next: "/", Link: ls.LinkUserID != nil}
	if ls.Next != nil {
		done.Next = *ls.Next
	}
	if done.Link {
		done.Next = "/account?tab=login"
	}
	if cb.Error != "" || cb.Code == "" {
		return done, accesstypes.RefuseDenied
	}
	claims, err := s.oauth.Exchange(ctx, cb.Provider, cb.Code, ls.Verifier, ls.Nonce)
	if err != nil {
		return done, err
	}
	if utf8.RuneCountInString(claims.Subject) > accesstypes.MaxSubjectLen {
		return done, accesstypes.RefuseFailed
	}
	email := claimedEmail(&claims)
	identity, user, err := s.identityRepository.Find(ctx, cb.Provider, claims.Subject)
	if err != nil {
		return done, apperr.Wrap(err)
	}
	if done.Link {
		return done, s.linkIdentity(ctx, *ls.LinkUserID, identity, cb.Provider, claims.Subject, email)
	}
	var byEmail *accesstypes.User
	if identity == nil && email != nil {
		found, err := s.userRepository.FindCredentialsByEmail(ctx, *email)
		if err != nil {
			return done, apperr.Wrap(err)
		}
		if found != nil {
			byEmail = &found.User
		}
	}
	policy := accesstypes.ProviderPolicy{Key: cfg.Key, AutoProvision: cfg.AutoProvision, AllowedDomains: cfg.AllowedDomains}
	outcome, err := accesstypes.Decide(user, byEmail, claims, policy)
	if err != nil {
		return done, err
	}
	newIdentity := accesstypes.NewIdentity{ID: uuid.Must(uuid.NewV7()), Provider: cb.Provider, Subject: claims.Subject, Email: email}
	if user, err = s.applyOutcome(ctx, outcome, loginParties{identity: identity, user: user, byEmail: byEmail}, newIdentity, claims); err != nil {
		return done, err
	}
	groups := accesstypes.GroupsFor(claims.Groups, groupRules(cfg.GroupMap))
	editors := accesstypes.GroupsFor(claims.Groups, groupRules(cfg.EditorMap))
	if err := s.groupRepository.SyncManaged(ctx, user.ID, accesstypes.OAuthSource(cb.Provider), groups, editors); err != nil {
		return done, apperr.Wrap(err)
	}
	signedIn, err := s.auth.openSession(ctx, *user, accesstypes.OAuthMethod(cb.Provider), previousToken)
	if err != nil {
		return done, err
	}
	done.Token = signedIn.Token
	return done, nil
}

func (s *OAuthService) linkIdentity(ctx context.Context, userID uuid.UUID, existing *accesstypes.Identity, provider, subject string, email *string) error {
	if existing != nil {
		if existing.UserID != userID {
			return accesstypes.RefuseIdentityTaken
		}
		return nil
	}
	_, err := s.identityRepository.Insert(ctx, accesstypes.NewIdentity{ID: uuid.Must(uuid.NewV7()), UserID: userID, Provider: provider,
		Subject: subject, Email: email})
	if refusal, ok := errors.AsType[accesstypes.Refusal](err); ok {
		return refusal
	}
	return apperr.Wrap(err)
}

func groupRules(rules []config.GroupRule) []accesstypes.GroupRule {
	out := make([]accesstypes.GroupRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, accesstypes.GroupRule{Value: r.Value, Group: r.Group})
	}
	return out
}

func (s *OAuthService) PruneLoginStates(ctx context.Context) (int64, error) {
	n, err := s.loginStateRepository.Prune(ctx)
	return n, apperr.Wrap(err)
}

type OAuthService struct {
	cfg                  *config.Config
	groupRepository      *repositories.GroupRepository
	identityRepository   *repositories.IdentityRepository
	loginStateRepository *repositories.LoginStateRepository
	oauth                accesstypes.OAuthClient
	userRepository       *repositories.UserRepository
	auth                 *AuthService
}

type loginParties struct {
	identity      *accesstypes.Identity
	user, byEmail *accesstypes.User
}

func claimedEmail(claims *accesstypes.Claims) *string {
	if utf8.RuneCountInString(claims.Email) > accesstypes.MaxEmailLen {
		claims.Email = ""
	}
	if claims.Email == "" {
		return nil
	}
	return new(strings.ToLower(strings.TrimSpace(claims.Email)))
}

func (s *OAuthService) applyOutcome(ctx context.Context, outcome accesstypes.Outcome, who loginParties, newIdentity accesstypes.NewIdentity, claims accesstypes.Claims) (*accesstypes.User, error) {
	switch outcome {
	case accesstypes.SignIn:
		if err := s.identityRepository.Touch(ctx, who.identity.ID, newIdentity.Email); err != nil {
			return nil, apperr.Wrap(err)
		}
		return who.user, nil
	case accesstypes.LinkAndSignIn:
		newIdentity.UserID = who.byEmail.ID
		if _, err := s.identityRepository.Insert(ctx, newIdentity); err != nil {
			return nil, apperr.Wrap(err)
		}
		return who.byEmail, nil
	case accesstypes.Provision:
		addr, err := accesstypes.ValidateEmail(*newIdentity.Email)
		if err != nil {
			return nil, accesstypes.RefuseNotProvisioned
		}
		created, err := s.identityRepository.CreateUser(ctx, accesstypes.NewUser{ID: uuid.Must(uuid.NewV7()), Email: addr,
			DisplayName: accesstypes.ProvisionedName(claims)}, newIdentity)
		if err != nil {
			return nil, apperr.Wrap(err)
		}
		return &created, nil
	}
	return who.user, nil
}
