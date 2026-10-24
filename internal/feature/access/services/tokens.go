package services

import (
	"context"
	"errors"

	"github.com/google/uuid"

	accessports "svc-registry/internal/feature/access/ports"
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
	return s.issueFor(ctx, accesstypes.User{ID: p.UserID, IsSuperadmin: p.IsSuperadmin}, in)
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
	return s.issueFor(ctx, *user, in)
}

func (s *TokensService) issueFor(ctx context.Context, owner accesstypes.User, in accesstypes.CreateToken) (IssuedToken, error) {
	userID, superadmin := owner.ID, owner.IsSuperadmin
	grantor := accesstypes.Grantor{Superadmin: superadmin}
	if !superadmin {
		rights, err := s.permissionRepository.ForUser(ctx, userID)
		if err != nil {
			return IssuedToken{}, apperr.Wrap(err)
		}
		grantor.Rights = rights.Canonical()
	}
	valid, err := accesstypes.ValidateCreateToken(in, grantor, s.cfg.Pat.MaxLifetimeDays)
	if err != nil {
		return IssuedToken{}, apperr.Wrap(err)
	}
	if valid.Nodes != nil {
		readable, err := s.roots.CheckRoots(ctx, accesstypes.Principal{UserID: userID, IsSuperadmin: superadmin}, valid.Nodes)
		if err != nil {
			return IssuedToken{}, apperr.Wrap(err)
		}
		if !readable {
			return IssuedToken{}, apperr.Wrap(accesstypes.InvalidTokenNodes)
		}
	}
	fresh, err := auth.GeneratePersonalToken()
	if err != nil {
		return IssuedToken{}, apperr.Internalf("no randomness: %v", err)
	}
	token, err := s.tokenRepository.Insert(ctx, accesstypes.NewToken{
		ID: uuid.Must(uuid.NewV7()), UserID: userID, Name: valid.Name, Prefix: fresh.Prefix,
		Hash: fresh.Hash, Scopes: valid.Scopes, ExpiresInDays: valid.ExpiresInDays, Nodes: valid.Nodes,
	})
	if err != nil {
		return IssuedToken{}, apperr.Wrap(err)
	}
	described, err := s.describe(ctx, []accesstypes.Token{token})
	if err != nil {
		return IssuedToken{}, err
	}
	return IssuedToken{Token: described[0], Secret: fresh.Secret}, nil
}

func (s *TokensService) describe(ctx context.Context, tokens []accesstypes.Token) ([]accesstypes.Token, error) {
	sets := make([][]uuid.UUID, len(tokens))
	for i, t := range tokens {
		sets[i] = t.NodeIDs
	}
	refs, err := nodeRefs(ctx, s.roots, sets)
	if err != nil {
		return nil, err
	}
	for i := range tokens {
		tokens[i].Nodes = refs[i]
	}
	return tokens, nil
}

func (s *TokensService) ListOwnTokens(ctx context.Context, p accesstypes.Principal) ([]accesstypes.Token, error) {
	if err := requireSession(p); err != nil {
		return nil, err
	}
	tokens, err := s.tokenRepository.ListForUser(ctx, p.UserID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.describe(ctx, tokens)
}

func (s *TokensService) RevokeOwnToken(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if err := requireSession(p); err != nil {
		return err
	}
	return apperr.Wrap(s.tokenRepository.Revoke(ctx, id, new(p.UserID)))
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
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.describe(ctx, tokens)
}

func (s *TokensService) ListTokens(ctx context.Context, p accesstypes.Principal, q accesstypes.TokenQuery) (accesstypes.Page[accesstypes.Token], error) {
	if err := requireRightSession(p, accesstypes.ActionRead); err != nil {
		return accesstypes.Page[accesstypes.Token]{}, err
	}
	filter, err := accesstypes.ValidateTokenQuery(q)
	if err != nil {
		return accesstypes.Page[accesstypes.Token]{}, apperr.Wrap(err)
	}
	page, err := s.tokenRepository.List(ctx, filter)
	if err != nil {
		return accesstypes.Page[accesstypes.Token]{}, apperr.Wrap(err)
	}
	page.Items, err = s.describe(ctx, page.Items)
	return page, err
}

func (s *TokensService) RevokeToken(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if err := requireRightSession(p, accesstypes.ActionDelete); err != nil {
		return err
	}
	if !p.IsSuperadmin {
		if err := s.unprotectedOwner(ctx, id); err != nil {
			return err
		}
	}
	return apperr.Wrap(s.tokenRepository.Revoke(ctx, id, nil))
}

func (s *TokensService) unprotectedOwner(ctx context.Context, id uuid.UUID) error {
	owner, err := s.tokenRepository.Owner(ctx, id)
	if err != nil || owner == nil {
		return apperr.Wrap(err)
	}
	user, err := s.userRepository.Find(ctx, *owner)
	if err != nil || user == nil {
		return apperr.Wrap(err)
	}
	return protectedUser(ctx, s.permissionRepository, *user)
}

func requireRightSession(p accesstypes.Principal, action accesstypes.Action) error {
	if err := requireSession(p); err != nil {
		return err
	}
	return accesstypes.RequireRight(p, accesstypes.SectionTokens, action)
}

type TokensService struct {
	permissionRepository *repositories.GroupPermissionRepository
	cfg                  *config.Config
	roots                accessports.NodeRootsPort
	tokenRepository      *repositories.TokenRepository
	userRepository       *repositories.UserRepository
}

func (s *TokensService) AddTokenRoot(ctx context.Context, tokenID, nodeID uuid.UUID) (bool, error) {
	err := s.tokenRepository.AddNode(ctx, tokenID, nodeID)
	if errors.Is(err, apperr.ErrNotFound) {
		return false, nil
	}
	return err == nil, apperr.Wrap(err)
}
