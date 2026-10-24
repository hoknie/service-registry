package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/ownership/repositories"
	ownershiptypes "svc-registry/internal/feature/ownership/types"
	"svc-registry/internal/platform/apperr"
)

func (s *OwnersService) NodeOwners(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (ownershiptypes.OwnerSet, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, id); err != nil {
		return ownershiptypes.OwnerSet{}, err
	}
	return s.effective(ctx, id)
}

func (s *OwnersService) SetNodeOwners(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in []ownershiptypes.PutOwner, none bool) (ownershiptypes.OwnerSet, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return ownershiptypes.OwnerSet{}, err
	}
	owners, err := ownershiptypes.ValidateOwners(in, none)
	if err != nil {
		return ownershiptypes.OwnerSet{}, apperr.Wrap(err)
	}
	if err := s.ownerRepository.Replace(ctx, id, owners, none); err != nil {
		return ownershiptypes.OwnerSet{}, apperr.Wrap(err)
	}
	return s.effective(ctx, id)
}

func (s *OwnersService) OwnerCandidates(ctx context.Context, p accesstypes.Principal, id uuid.UUID, raw string) ([]ownershiptypes.Candidate, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return nil, err
	}
	q, err := ownershiptypes.ValidateCandidateQuery(raw)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out, err := s.ownerRepository.Candidates(ctx, id, catalogtypes.LikePrefix(q))
	return out, apperr.Wrap(err)
}

func (s *OwnersService) SuggestOwners(ctx context.Context, p accesstypes.Principal, raw string) ([]ownershiptypes.Suggestion, error) {
	q, err := ownershiptypes.ValidateSuggestQuery(raw)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out, err := s.ownerRepository.Suggest(ctx, catalogtypes.Viewer{UserID: p.UserID, Nodes: p.Nodes, Admin: p.CatalogAdmin}, p.IsSuperadmin, catalogtypes.LikePrefix(q))
	return out, apperr.Wrap(err)
}

func (s *OwnersService) OwnerBadges(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]ownershiptypes.Badge, error) {
	out, err := s.ownerRepository.Badges(ctx, ids)
	return out, apperr.Wrap(err)
}

func (s *OwnersService) effective(ctx context.Context, id uuid.UUID) (ownershiptypes.OwnerSet, error) {
	src, items, err := s.ownerRepository.Effective(ctx, id)
	if err != nil {
		return ownershiptypes.OwnerSet{}, apperr.Wrap(err)
	}
	return ownershiptypes.SetOf(id, src, items), nil
}

type OwnersService struct {
	catalog         *catalogservices.CatalogServices
	ownerRepository *repositories.OwnerRepository
}
