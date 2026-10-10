package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/platform/apperr"
)

type IdentityView struct {
	access.Identity
	DisplayName string
}

func (s *Service) identityViews(items []access.Identity) []IdentityView {
	out := make([]IdentityView, 0, len(items))
	for _, i := range items {
		name := i.Provider
		if cfg, ok := s.providerConfig(i.Provider); ok {
			name = cfg.DisplayName
		}
		out = append(out, IdentityView{Identity: i, DisplayName: name})
	}
	return out
}

func (s *Service) OwnIdentities(ctx context.Context, p access.Principal) ([]IdentityView, error) {
	items, err := s.identities.ListForUser(ctx, p.UserID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.identityViews(items), nil
}

func (s *Service) UserIdentities(ctx context.Context, p access.Principal, userID uuid.UUID) ([]IdentityView, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return nil, err
	}
	user, err := s.users.Find(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if user == nil {
		return nil, apperr.New(apperr.NotFound)
	}
	items, err := s.identities.ListForUser(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.identityViews(items), nil
}

func (s *Service) UnlinkIdentity(ctx context.Context, p access.Principal, id uuid.UUID) error {
	return apperr.Wrap(s.identities.Delete(ctx, p.UserID, id))
}
