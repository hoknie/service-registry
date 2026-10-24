package services

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
)

type IdentityView struct {
	accesstypes.Identity
	DisplayName string
}

func (s *IdentitiesService) identityViews(items []accesstypes.Identity) []IdentityView {
	out := make([]IdentityView, 0, len(items))
	for _, i := range items {
		name := i.Provider
		if cfg, ok := s.oauthService.providerConfig(i.Provider); ok {
			name = cfg.DisplayName
		}
		out = append(out, IdentityView{Identity: i, DisplayName: name})
	}
	return out
}

func (s *IdentitiesService) OwnIdentities(ctx context.Context, p accesstypes.Principal) ([]IdentityView, error) {
	items, err := s.identityRepository.ListForUser(ctx, p.UserID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.identityViews(items), nil
}

func (s *IdentitiesService) UserIdentities(ctx context.Context, p accesstypes.Principal, userID uuid.UUID) ([]IdentityView, error) {
	if err := accesstypes.RequireRight(p, accesstypes.SectionUsers, accesstypes.ActionRead); err != nil {
		return nil, err
	}
	user, err := s.userRepository.Find(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if user == nil {
		return nil, apperr.New(apperr.NotFound)
	}
	items, err := s.identityRepository.ListForUser(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.identityViews(items), nil
}

func (s *IdentitiesService) UnlinkIdentity(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	return apperr.Wrap(s.identityRepository.Delete(ctx, p.UserID, id))
}

type IdentitiesService struct {
	identityRepository *repositories.IdentityRepository
	userRepository     *repositories.UserRepository
	oauthService       *OAuthService
}
