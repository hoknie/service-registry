package service

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
	"svc-registry/internal/auth"
)

type SignedIn struct {
	User  access.User
	Token string
}

func Login(ctx context.Context, state *State, ip string, creds access.Credentials, previousToken *string) (SignedIn, error) {
	key := strings.ToLower(strings.TrimFunc(creds.Email, unicode.IsSpace))
	if secs, ok := state.LoginLimiter.Check(ip, key, time.Now()); !ok {
		return SignedIn{}, apperr.RateLimitedFor(secs)
	}
	found, err := state.Users.FindCredentialsByEmail(ctx, key)
	if err != nil {
		return SignedIn{}, apperr.Wrap(err)
	}
	mode := access.PasswordLogin(state.Config.OAuth.PasswordLogin)
	if found == nil || found.PasswordHash == "" {
		state.Hasher.VerifyDummy(creds.Password)
		state.LoginLimiter.RecordFailure(ip, key, time.Now())
		return SignedIn{}, apperr.New(apperr.InvalidCredentials)
	}
	if !state.Hasher.Verify(creds.Password, found.PasswordHash) || !mode.Allows(found.User.IsSuperadmin) {
		state.LoginLimiter.RecordFailure(ip, key, time.Now())
		return SignedIn{}, apperr.New(apperr.InvalidCredentials)
	}
	if found.User.Status != access.StatusActive {
		return SignedIn{}, apperr.New(apperr.AccountDisabled)
	}
	state.LoginLimiter.Reset(ip, key)
	return openSession(ctx, state, found.User, access.MethodPassword, previousToken)
}

func openSession(ctx context.Context, state *State, user access.User, method string, previousToken *string) (SignedIn, error) {
	if previousToken != nil {
		if err := state.Sessions.DeleteByToken(ctx, auth.TokenHash(*previousToken)); err != nil {
			return SignedIn{}, apperr.Wrap(err)
		}
	}
	cfg := state.Config.Session
	if _, err := state.Sessions.DeleteExpiredForUser(ctx, user.ID, cfg.IdleTimeoutSecs); err != nil {
		return SignedIn{}, apperr.Wrap(err)
	}
	token, err := auth.NewSessionToken()
	if err != nil {
		return SignedIn{}, apperr.Internalf("no randomness: %v", err)
	}
	err = state.Sessions.Create(ctx, access.NewSession{
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

func Logout(ctx context.Context, state *State, token *string) error {
	if token == nil {
		return nil
	}
	return apperr.Wrap(state.Sessions.DeleteByToken(ctx, auth.TokenHash(*token)))
}

func Authenticate(ctx context.Context, state *State, token string) (Principal, access.User, error) {
	found, err := state.Sessions.FindValid(ctx, auth.TokenHash(token), state.Config.Session.IdleTimeoutSecs)
	if err != nil {
		return Principal{}, access.User{}, apperr.Wrap(err)
	}
	if found == nil {
		return Principal{}, access.User{}, apperr.New(apperr.Unauthenticated)
	}
	p := Principal{UserID: found.User.ID, SessionID: found.SessionID, IsSuperadmin: found.User.IsSuperadmin, Method: found.Method}
	return p, found.User, nil
}

func AuthenticateToken(ctx context.Context, state *State, secret string) (Principal, access.User, error) {
	hash, ok := auth.ParsePersonalToken(secret)
	if !ok {
		return Principal{}, access.User{}, apperr.New(apperr.InvalidToken)
	}
	found, err := state.Tokens.FindValid(ctx, hash)
	if err != nil {
		return Principal{}, access.User{}, apperr.Wrap(err)
	}
	if found == nil {
		return Principal{}, access.User{}, apperr.New(apperr.InvalidToken)
	}
	p := Principal{
		UserID:       found.User.ID,
		TokenID:      found.TokenID,
		Scopes:       found.Scopes,
		IsSuperadmin: found.User.IsSuperadmin && found.Scopes.Has(access.ScopeAdmin),
		Method:       "token",
	}
	return p, found.User, nil
}

func ChangePassword(ctx context.Context, state *State, p Principal, change access.PasswordChange) error {
	current, err := state.Users.FindCredentials(ctx, p.UserID)
	if err != nil {
		return apperr.Wrap(err)
	}
	if current == nil {
		return apperr.New(apperr.Unauthenticated)
	}
	switch {
	case change.CurrentPassword == nil && current.PasswordHash != "":
		return &apperr.Error{Kind: apperr.Validation, Code: "validation.invalid_body", Message: "current_password is required"}
	case change.CurrentPassword != nil && (current.PasswordHash == "" || !state.Hasher.Verify(*change.CurrentPassword, current.PasswordHash)):
		return apperr.New(apperr.WrongPassword)
	}
	if err := access.ValidatePassword(change.NewPassword); err != nil {
		return apperr.Wrap(err)
	}
	hash, err := state.Hasher.Hash(change.NewPassword)
	if err != nil {
		return apperr.Internalf("%v", err)
	}
	if err := state.Users.SetPassword(ctx, p.UserID, hash); err != nil {
		return apperr.Wrap(err)
	}
	keep := p.SessionID
	_, err = state.Sessions.DeleteForUser(ctx, p.UserID, &keep)
	return apperr.Wrap(err)
}
