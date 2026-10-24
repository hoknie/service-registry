package services

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
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
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		hasPassword, err := s.userRepository.LockPassword(ctx, p.UserID)
		if err != nil {
			return err
		}
		if hasPassword == nil {
			return apperr.ErrNotFound
		}
		count, err := s.identityRepository.CountForUser(ctx, p.UserID)
		if err != nil {
			return err
		}
		if err := s.identityRepository.Delete(ctx, p.UserID, id); err != nil {
			return err
		}
		if accesstypes.LastLoginMethod(*hasPassword, count) {
			return accesstypes.ConflictLastLoginMethod
		}
		return nil
	})
	return apperr.Wrap(err)
}

type IdentitiesService struct {
	db                 *pgtx.DB
	identityRepository *repositories.IdentityRepository
	userRepository     *repositories.UserRepository
	oauthService       *OAuthService
}
