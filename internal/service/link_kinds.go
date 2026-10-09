package service

import (
	"context"

	"svc-registry/internal/apperr"
	"svc-registry/internal/links"
)

func ListLinkKinds(ctx context.Context, state *State, _ Principal) ([]links.Kind, error) {
	items, err := state.LinkKinds.List(ctx)
	return items, apperr.Wrap(err)
}

func CreateLinkKind(ctx context.Context, state *State, p Principal, in links.CreateKind) (links.Kind, error) {
	if err := RequireSuperadmin(p); err != nil {
		return links.Kind{}, err
	}
	k, err := links.ValidateNewKind(in)
	if err != nil {
		return links.Kind{}, apperr.Wrap(err)
	}
	created, err := state.LinkKinds.Insert(ctx, k)
	return created, apperr.Wrap(err)
}

func UpdateLinkKind(ctx context.Context, state *State, p Principal, rawKey string, in links.UpdateKind) (links.Kind, error) {
	if err := RequireSuperadmin(p); err != nil {
		return links.Kind{}, err
	}
	key, err := links.ValidateKey(rawKey)
	if err != nil {
		return links.Kind{}, apperr.New(apperr.NotFound)
	}
	changes, err := links.ValidateKindChanges(in)
	if err != nil {
		return links.Kind{}, apperr.Wrap(err)
	}
	k, err := state.LinkKinds.Update(ctx, key, changes)
	if err != nil {
		return links.Kind{}, apperr.Wrap(err)
	}
	if k == nil {
		return links.Kind{}, apperr.New(apperr.NotFound)
	}
	return *k, nil
}

func DeleteLinkKind(ctx context.Context, state *State, p Principal, rawKey string) error {
	if err := RequireSuperadmin(p); err != nil {
		return err
	}
	key, err := links.ValidateKey(rawKey)
	if err != nil {
		return apperr.New(apperr.NotFound)
	}
	return apperr.Wrap(state.LinkKinds.Delete(ctx, key))
}
