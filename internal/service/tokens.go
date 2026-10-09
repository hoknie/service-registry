package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
	"svc-registry/internal/auth"
)

type IssuedToken struct {
	Token  access.Token
	Secret string
}

func requireSession(p Principal) error {
	if p.ByToken() {
		return apperr.New(apperr.SessionRequired)
	}
	return nil
}

func requireSuperadminSession(p Principal) error {
	if err := requireSession(p); err != nil {
		return err
	}
	return RequireSuperadmin(p)
}

func IssueToken(ctx context.Context, state *State, p Principal, in access.CreateToken) (IssuedToken, error) {
	if err := requireSession(p); err != nil {
		return IssuedToken{}, err
	}
	return issueFor(ctx, state, p.UserID, p.IsSuperadmin, in)
}

func IssueUserToken(ctx context.Context, state *State, p Principal, userID uuid.UUID, in access.CreateToken) (IssuedToken, error) {
	if err := requireSession(p); err != nil {
		return IssuedToken{}, err
	}
	if err := RequireSuperadmin(p); err != nil {
		return IssuedToken{}, err
	}
	user, err := state.Users.Find(ctx, userID)
	if err != nil {
		return IssuedToken{}, apperr.Wrap(err)
	}
	if user == nil {
		return IssuedToken{}, apperr.New(apperr.NotFound)
	}
	if user.Status != access.StatusActive {
		return IssuedToken{}, apperr.Wrap(access.ConflictUserDisabled)
	}
	return issueFor(ctx, state, user.ID, user.IsSuperadmin, in)
}

func issueFor(ctx context.Context, state *State, userID uuid.UUID, superadmin bool, in access.CreateToken) (IssuedToken, error) {
	valid, err := access.ValidateCreateToken(in, superadmin, state.Config.Pat.MaxLifetimeDays)
	if err != nil {
		return IssuedToken{}, apperr.Wrap(err)
	}
	fresh, err := auth.GeneratePersonalToken()
	if err != nil {
		return IssuedToken{}, apperr.Internalf("no randomness: %v", err)
	}
	token, err := state.Tokens.Insert(ctx, access.NewToken{
		ID: uuid.Must(uuid.NewV7()), UserID: userID, Name: valid.Name, Prefix: fresh.Prefix,
		Hash: fresh.Hash, Scopes: valid.Scopes, ExpiresInDays: valid.ExpiresInDays,
	})
	if err != nil {
		return IssuedToken{}, apperr.Wrap(err)
	}
	return IssuedToken{Token: token, Secret: fresh.Secret}, nil
}

func ListOwnTokens(ctx context.Context, state *State, p Principal) ([]access.Token, error) {
	if err := requireSession(p); err != nil {
		return nil, err
	}
	tokens, err := state.Tokens.ListForUser(ctx, p.UserID)
	return tokens, apperr.Wrap(err)
}

func RevokeOwnToken(ctx context.Context, state *State, p Principal, id uuid.UUID) error {
	if err := requireSession(p); err != nil {
		return err
	}
	owner := p.UserID
	return apperr.Wrap(state.Tokens.Revoke(ctx, id, &owner))
}

func ListUserTokens(ctx context.Context, state *State, p Principal, userID uuid.UUID) ([]access.Token, error) {
	if err := requireSuperadminSession(p); err != nil {
		return nil, err
	}
	user, err := state.Users.Find(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if user == nil {
		return nil, apperr.New(apperr.NotFound)
	}
	tokens, err := state.Tokens.ListForUser(ctx, userID)
	return tokens, apperr.Wrap(err)
}

func ListTokens(ctx context.Context, state *State, p Principal, q access.TokenQuery) (access.Page[access.Token], error) {
	if err := requireSuperadminSession(p); err != nil {
		return access.Page[access.Token]{}, err
	}
	filter, err := access.ValidateTokenQuery(q)
	if err != nil {
		return access.Page[access.Token]{}, apperr.Wrap(err)
	}
	page, err := state.Tokens.List(ctx, filter)
	return page, apperr.Wrap(err)
}

func RevokeToken(ctx context.Context, state *State, p Principal, id uuid.UUID) error {
	if err := requireSuperadminSession(p); err != nil {
		return err
	}
	return apperr.Wrap(state.Tokens.Revoke(ctx, id, nil))
}
