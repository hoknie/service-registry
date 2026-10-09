package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
)

type IdentityView struct {
	access.Identity
	DisplayName string
}

func identityViews(state *State, items []access.Identity) []IdentityView {
	out := make([]IdentityView, 0, len(items))
	for _, i := range items {
		name := i.Provider
		if cfg, ok := providerConfig(state, i.Provider); ok {
			name = cfg.DisplayName
		}
		out = append(out, IdentityView{Identity: i, DisplayName: name})
	}
	return out
}

func OwnIdentities(ctx context.Context, state *State, p Principal) ([]IdentityView, error) {
	items, err := state.Identities.ListForUser(ctx, p.UserID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return identityViews(state, items), nil
}

func UserIdentities(ctx context.Context, state *State, p Principal, userID uuid.UUID) ([]IdentityView, error) {
	if err := RequireSuperadmin(p); err != nil {
		return nil, err
	}
	user, err := state.Users.Find(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if user == nil {
		return nil, apperr.New(apperr.NotFound)
	}
	items, err := state.Identities.ListForUser(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return identityViews(state, items), nil
}

func UnlinkIdentity(ctx context.Context, state *State, p Principal, id uuid.UUID) error {
	return apperr.Wrap(state.Identities.Delete(ctx, p.UserID, id))
}
