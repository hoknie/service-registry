package service

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
)

type SignedIn struct {
	User  access.User
	Token string
}

func (s *Service) Login(ctx context.Context, ip string, creds access.Credentials, previousToken *string) (SignedIn, error) {
	key := strings.ToLower(strings.TrimFunc(creds.Email, unicode.IsSpace))
	if secs, ok := s.loginLimiter.Check(ip, key, time.Now()); !ok {
		return SignedIn{}, apperr.RateLimitedFor(secs)
	}
	found, err := s.users.FindCredentialsByEmail(ctx, key)
	if err != nil {
		return SignedIn{}, apperr.Wrap(err)
	}
	mode := access.PasswordLogin(s.cfg.OAuth.PasswordLogin)
	if found == nil || found.PasswordHash == "" {
		s.hasher.VerifyDummy(creds.Password)
		s.loginLimiter.RecordFailure(ip, key, time.Now())
		return SignedIn{}, apperr.New(apperr.InvalidCredentials)
	}
	if !s.hasher.Verify(creds.Password, found.PasswordHash) || !mode.Allows(found.User.IsSuperadmin) || found.User.IsService {
		s.loginLimiter.RecordFailure(ip, key, time.Now())
		return SignedIn{}, apperr.New(apperr.InvalidCredentials)
	}
	if found.User.Status != access.StatusActive {
		return SignedIn{}, apperr.New(apperr.AccountDisabled)
	}
	s.loginLimiter.Reset(ip, key)
	return s.openSession(ctx, found.User, access.MethodPassword, previousToken)
}

func (s *Service) openSession(ctx context.Context, user access.User, method string, previousToken *string) (SignedIn, error) {
	if previousToken != nil {
		if err := s.sessions.DeleteByToken(ctx, auth.TokenHash(*previousToken)); err != nil {
			return SignedIn{}, apperr.Wrap(err)
		}
	}
	cfg := s.cfg.Session
	if _, err := s.sessions.DeleteExpiredForUser(ctx, user.ID, cfg.IdleTimeoutSecs); err != nil {
		return SignedIn{}, apperr.Wrap(err)
	}
	token, err := auth.NewSessionToken()
	if err != nil {
		return SignedIn{}, apperr.Internalf("no randomness: %v", err)
	}
	err = s.sessions.Create(ctx, access.NewSession{
		ID:                  uuid.Must(uuid.NewV7()),
		UserID:              user.ID,
		TokenHash:           token.Hash,
		AbsoluteTimeoutSecs: cfg.AbsoluteTimeoutSecs,
		Method:              method,
	})
	if err != nil {
		return SignedIn{}, apperr.Wrap(err)
	}
	return SignedIn{User: user, Token: token.Token}, nil
}

func (s *Service) Logout(ctx context.Context, token *string) error {
	if token == nil {
		return nil
	}
	return apperr.Wrap(s.sessions.DeleteByToken(ctx, auth.TokenHash(*token)))
}

func (s *Service) Authenticate(ctx context.Context, token string) (access.Principal, access.User, error) {
	found, err := s.sessions.FindValid(ctx, auth.TokenHash(token), s.cfg.Session.IdleTimeoutSecs)
	if err != nil {
		return access.Principal{}, access.User{}, apperr.Wrap(err)
	}
	if found == nil {
		return access.Principal{}, access.User{}, apperr.New(apperr.Unauthenticated)
	}
	p := access.Principal{UserID: found.User.ID, SessionID: found.SessionID, IsSuperadmin: found.User.IsSuperadmin, Method: found.Method}
	return p, found.User, nil
}

func (s *Service) AuthenticateToken(ctx context.Context, secret string) (access.Principal, access.User, error) {
	hash, ok := auth.ParsePersonalToken(secret)
	if !ok {
		return access.Principal{}, access.User{}, apperr.New(apperr.InvalidToken)
	}
	found, err := s.tokens.FindValid(ctx, hash)
	if err != nil {
		return access.Principal{}, access.User{}, apperr.Wrap(err)
	}
	if found == nil {
		return access.Principal{}, access.User{}, apperr.New(apperr.InvalidToken)
	}
	p := access.Principal{
		UserID:       found.User.ID,
		TokenID:      found.TokenID,
		Scopes:       found.Scopes,
		IsSuperadmin: found.User.IsSuperadmin && found.Scopes.Has(access.ScopeAdmin),
		Method:       "token",
	}
	return p, found.User, nil
}

func (s *Service) ChangePassword(ctx context.Context, p access.Principal, change access.PasswordChange) error {
	current, err := s.users.FindCredentials(ctx, p.UserID)
	if err != nil {
		return apperr.Wrap(err)
	}
	if current == nil {
		return apperr.New(apperr.Unauthenticated)
	}
	switch {
	case change.CurrentPassword == nil && current.PasswordHash != "":
		return &apperr.Error{Kind: apperr.Validation, Code: "validation.invalid_body", Message: "current_password is required"}
	case change.CurrentPassword != nil && (current.PasswordHash == "" || !s.hasher.Verify(*change.CurrentPassword, current.PasswordHash)):
		return apperr.New(apperr.WrongPassword)
	}
	if err := access.ValidatePassword(change.NewPassword); err != nil {
		return apperr.Wrap(err)
	}
	hash, err := s.hasher.Hash(change.NewPassword)
	if err != nil {
		return apperr.Internalf("%v", err)
	}
	if err := s.users.SetPassword(ctx, p.UserID, hash); err != nil {
		return apperr.Wrap(err)
	}
	keep := p.SessionID
	_, err = s.sessions.DeleteForUser(ctx, p.UserID, &keep)
	return apperr.Wrap(err)
}
