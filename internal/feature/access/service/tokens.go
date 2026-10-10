package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
)

type IssuedToken struct {
	Token  access.Token
	Secret string
}

func requireSession(p access.Principal) error {
	if p.ByToken() {
		return apperr.New(apperr.SessionRequired)
	}
	return nil
}

func requireSuperadminSession(p access.Principal) error {
	if err := requireSession(p); err != nil {
		return err
	}
	return access.RequireSuperadmin(p)
}

func (s *Service) IssueToken(ctx context.Context, p access.Principal, in access.CreateToken) (IssuedToken, error) {
	if err := requireSession(p); err != nil {
		return IssuedToken{}, err
	}
	return s.issueFor(ctx, p.UserID, p.IsSuperadmin, in)
}

func (s *Service) IssueUserToken(ctx context.Context, p access.Principal, userID uuid.UUID, in access.CreateToken) (IssuedToken, error) {
	if err := requireSession(p); err != nil {
		return IssuedToken{}, err
	}
	if err := access.RequireSuperadmin(p); err != nil {
		return IssuedToken{}, err
	}
	user, err := s.users.Find(ctx, userID)
	if err != nil {
		return IssuedToken{}, apperr.Wrap(err)
	}
	if user == nil {
		return IssuedToken{}, apperr.New(apperr.NotFound)
	}
	if user.Status != access.StatusActive {
		return IssuedToken{}, apperr.Wrap(access.ConflictUserDisabled)
	}
	return s.issueFor(ctx, user.ID, user.IsSuperadmin, in)
}

func (s *Service) issueFor(ctx context.Context, userID uuid.UUID, superadmin bool, in access.CreateToken) (IssuedToken, error) {
	valid, err := access.ValidateCreateToken(in, superadmin, s.cfg.Pat.MaxLifetimeDays)
	if err != nil {
		return IssuedToken{}, apperr.Wrap(err)
	}
	fresh, err := auth.GeneratePersonalToken()
	if err != nil {
		return IssuedToken{}, apperr.Internalf("no randomness: %v", err)
	}
	token, err := s.tokens.Insert(ctx, access.NewToken{
		ID: uuid.Must(uuid.NewV7()), UserID: userID, Name: valid.Name, Prefix: fresh.Prefix,
		Hash: fresh.Hash, Scopes: valid.Scopes, ExpiresInDays: valid.ExpiresInDays,
	})
	if err != nil {
		return IssuedToken{}, apperr.Wrap(err)
	}
	return IssuedToken{Token: token, Secret: fresh.Secret}, nil
}

func (s *Service) ListOwnTokens(ctx context.Context, p access.Principal) ([]access.Token, error) {
	if err := requireSession(p); err != nil {
		return nil, err
	}
	tokens, err := s.tokens.ListForUser(ctx, p.UserID)
	return tokens, apperr.Wrap(err)
}

func (s *Service) RevokeOwnToken(ctx context.Context, p access.Principal, id uuid.UUID) error {
	if err := requireSession(p); err != nil {
		return err
	}
	owner := p.UserID
	return apperr.Wrap(s.tokens.Revoke(ctx, id, &owner))
}

func (s *Service) ListUserTokens(ctx context.Context, p access.Principal, userID uuid.UUID) ([]access.Token, error) {
	if err := requireSuperadminSession(p); err != nil {
		return nil, err
	}
	user, err := s.users.Find(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if user == nil {
		return nil, apperr.New(apperr.NotFound)
	}
	tokens, err := s.tokens.ListForUser(ctx, userID)
	return tokens, apperr.Wrap(err)
}

func (s *Service) ListTokens(ctx context.Context, p access.Principal, q access.TokenQuery) (access.Page[access.Token], error) {
	if err := requireSuperadminSession(p); err != nil {
		return access.Page[access.Token]{}, err
	}
	filter, err := access.ValidateTokenQuery(q)
	if err != nil {
		return access.Page[access.Token]{}, apperr.Wrap(err)
	}
	page, err := s.tokens.List(ctx, filter)
	return page, apperr.Wrap(err)
}

func (s *Service) RevokeToken(ctx context.Context, p access.Principal, id uuid.UUID) error {
	if err := requireSuperadminSession(p); err != nil {
		return err
	}
	return apperr.Wrap(s.tokens.Revoke(ctx, id, nil))
}
