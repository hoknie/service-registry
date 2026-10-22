package services

import (
	"context"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/links/repositories"
	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/apperr"
)

func (s *KindsService) ListLinkKinds(ctx context.Context, _ accesstypes.Principal) ([]linkstypes.Kind, error) {
	items, err := s.kindRepository.List(ctx)
	return items, apperr.Wrap(err)
}

func (s *KindsService) CreateLinkKind(ctx context.Context, p accesstypes.Principal, in linkstypes.CreateKind) (linkstypes.Kind, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return linkstypes.Kind{}, err
	}
	k, err := linkstypes.ValidateNewKind(in)
	if err != nil {
		return linkstypes.Kind{}, apperr.Wrap(err)
	}
	created, err := s.kindRepository.Insert(ctx, k)
	return created, apperr.Wrap(err)
}

func (s *KindsService) UpdateLinkKind(ctx context.Context, p accesstypes.Principal, rawKey string, in linkstypes.UpdateKind) (linkstypes.Kind, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return linkstypes.Kind{}, err
	}
	key, err := linkstypes.ValidateKey(rawKey)
	if err != nil {
		return linkstypes.Kind{}, apperr.New(apperr.NotFound)
	}
	changes, err := linkstypes.ValidateKindChanges(in)
	if err != nil {
		return linkstypes.Kind{}, apperr.Wrap(err)
	}
	k, err := s.kindRepository.Update(ctx, key, changes)
	if err != nil {
		return linkstypes.Kind{}, apperr.Wrap(err)
	}
	if k == nil {
		return linkstypes.Kind{}, apperr.New(apperr.NotFound)
	}
	return *k, nil
}

func (s *KindsService) DeleteLinkKind(ctx context.Context, p accesstypes.Principal, rawKey string) error {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return err
	}
	key, err := linkstypes.ValidateKey(rawKey)
	if err != nil {
		return apperr.New(apperr.NotFound)
	}
	return apperr.Wrap(s.kindRepository.Delete(ctx, key))
}

func (s *KindsService) ReorderLinkKinds(ctx context.Context, p accesstypes.Principal, rawKeys []string) ([]linkstypes.Kind, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return nil, err
	}
	keys, err := linkstypes.ValidateOrder(rawKeys)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if err := s.kindRepository.Reorder(ctx, keys); err != nil {
		return nil, apperr.Wrap(err)
	}
	items, err := s.kindRepository.List(ctx)
	return items, apperr.Wrap(err)
}

type KindsService struct {
	kindRepository *repositories.KindRepository
}
