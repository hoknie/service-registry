package services

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
)

type IssuedToken struct {
	Token  accesstypes.Token
	Secret string
}

func requireSession(p accesstypes.Principal) error {
	if p.ByToken() {
		return apperr.New(apperr.SessionRequired)
	}
	return nil
}

func requireSuperadminSession(p accesstypes.Principal) error {
	if err := requireSession(p); err != nil {
		return err
	}
	return accesstypes.RequireSuperadmin(p)
}

func (s *TokensService) IssueToken(ctx context.Context, p accesstypes.Principal, in accesstypes.CreateToken) (IssuedToken, error) {
	if err := requireSession(p); err != nil {
		return IssuedToken{}, err
	}
	return s.issueFor(ctx, p.UserID, p.IsSuperadmin, in)
}

func (s *TokensService) IssueUserToken(ctx context.Context, p accesstypes.Principal, userID uuid.UUID, in accesstypes.CreateToken) (IssuedToken, error) {
	if err := requireSession(p); err != nil {
		return IssuedToken{}, err
	}
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return IssuedToken{}, err
	}
	user, err := s.userRepository.Find(ctx, userID)
	if err != nil {
		return IssuedToken{}, apperr.Wrap(err)
	}
	if user == nil {
		return IssuedToken{}, apperr.New(apperr.NotFound)
	}
	if user.Status != accesstypes.StatusActive {
		return IssuedToken{}, apperr.Wrap(accesstypes.ConflictUserDisabled)
	}
	return s.issueFor(ctx, user.ID, user.IsSuperadmin, in)
}

func (s *TokensService) issueFor(ctx context.Context, userID uuid.UUID, superadmin bool, in accesstypes.CreateToken) (IssuedToken, error) {
	valid, err := accesstypes.ValidateCreateToken(in, superadmin, s.cfg.Pat.MaxLifetimeDays)
	if err != nil {
		return IssuedToken{}, apperr.Wrap(err)
	}
	fresh, err := auth.GeneratePersonalToken()
	if err != nil {
		return IssuedToken{}, apperr.Internalf("no randomness: %v", err)
	}
	token, err := s.tokenRepository.Insert(ctx, accesstypes.NewToken{
		ID: uuid.Must(uuid.NewV7()), UserID: userID, Name: valid.Name, Prefix: fresh.Prefix,
		Hash: fresh.Hash, Scopes: valid.Scopes, ExpiresInDays: valid.ExpiresInDays,
	})
	if err != nil {
		return IssuedToken{}, apperr.Wrap(err)
	}
	return IssuedToken{Token: token, Secret: fresh.Secret}, nil
}

func (s *TokensService) ListOwnTokens(ctx context.Context, p accesstypes.Principal) ([]accesstypes.Token, error) {
	if err := requireSession(p); err != nil {
		return nil, err
	}
	tokens, err := s.tokenRepository.ListForUser(ctx, p.UserID)
	return tokens, apperr.Wrap(err)
}

func (s *TokensService) RevokeOwnToken(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if err := requireSession(p); err != nil {
		return err
	}
	owner := p.UserID
	return apperr.Wrap(s.tokenRepository.Revoke(ctx, id, &owner))
}

func (s *TokensService) ListUserTokens(ctx context.Context, p accesstypes.Principal, userID uuid.UUID) ([]accesstypes.Token, error) {
	if err := requireSuperadminSession(p); err != nil {
		return nil, err
	}
	user, err := s.userRepository.Find(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if user == nil {
		return nil, apperr.New(apperr.NotFound)
	}
	tokens, err := s.tokenRepository.ListForUser(ctx, userID)
	return tokens, apperr.Wrap(err)
}

func (s *TokensService) ListTokens(ctx context.Context, p accesstypes.Principal, q accesstypes.TokenQuery) (accesstypes.Page[accesstypes.Token], error) {
	if err := requireSuperadminSession(p); err != nil {
		return accesstypes.Page[accesstypes.Token]{}, err
	}
	filter, err := accesstypes.ValidateTokenQuery(q)
	if err != nil {
		return accesstypes.Page[accesstypes.Token]{}, apperr.Wrap(err)
	}
	page, err := s.tokenRepository.List(ctx, filter)
	return page, apperr.Wrap(err)
}

func (s *TokensService) RevokeToken(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if err := requireSuperadminSession(p); err != nil {
		return err
	}
	return apperr.Wrap(s.tokenRepository.Revoke(ctx, id, nil))
}

type TokensService struct {
	cfg             *config.Config
	tokenRepository *repositories.TokenRepository
	userRepository  *repositories.UserRepository
}
