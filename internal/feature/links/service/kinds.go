package service

import (
	"context"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/links"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) ListLinkKinds(ctx context.Context, _ access.Principal) ([]links.Kind, error) {
	items, err := s.kinds.List(ctx)
	return items, apperr.Wrap(err)
}

func (s *Service) CreateLinkKind(ctx context.Context, p access.Principal, in links.CreateKind) (links.Kind, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return links.Kind{}, err
	}
	k, err := links.ValidateNewKind(in)
	if err != nil {
		return links.Kind{}, apperr.Wrap(err)
	}
	created, err := s.kinds.Insert(ctx, k)
	return created, apperr.Wrap(err)
}

func (s *Service) UpdateLinkKind(ctx context.Context, p access.Principal, rawKey string, in links.UpdateKind) (links.Kind, error) {
	if err := access.RequireSuperadmin(p); err != nil {
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
	k, err := s.kinds.Update(ctx, key, changes)
	if err != nil {
		return links.Kind{}, apperr.Wrap(err)
	}
	if k == nil {
		return links.Kind{}, apperr.New(apperr.NotFound)
	}
	return *k, nil
}

func (s *Service) DeleteLinkKind(ctx context.Context, p access.Principal, rawKey string) error {
	if err := access.RequireSuperadmin(p); err != nil {
		return err
	}
	key, err := links.ValidateKey(rawKey)
	if err != nil {
		return apperr.New(apperr.NotFound)
	}
	return apperr.Wrap(s.kinds.Delete(ctx, key))
}
