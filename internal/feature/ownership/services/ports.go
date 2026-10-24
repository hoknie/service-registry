package services

import (
	"context"
	"slices"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/ownership/repositories"
	ownershiptypes "svc-registry/internal/feature/ownership/types"
	"svc-registry/internal/platform/apperr"
)

func (s *PortsService) OwnedProjects(ctx context.Context, caller uuid.UUID, f catalogtypes.OwnerFilter) ([]uuid.UUID, error) {
	out, err := s.ownerRepository.OwnedProjects(ctx, caller, f)
	return out, apperr.Wrap(err)
}

func (s *PortsService) EffectiveOwners(ctx context.Context, id uuid.UUID) ([]ownershiptypes.Owner, error) {
	out, err := s.ownerRepository.Effective(ctx, id)
	return out, apperr.Wrap(err)
}

func (s *PortsService) GroupNodes(ctx context.Context, groupIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	out, err := s.catalog.Bindings.GroupNodes(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	owned, err := s.ownerRepository.GroupOwned(ctx, groupIDs)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	for _, pair := range owned {
		if !slices.Contains(out[pair[0]], pair[1]) {
			out[pair[0]] = append(out[pair[0]], pair[1])
		}
	}
	return out, nil
}

func (s *PortsService) AddCreatorOwner(ctx context.Context, nodeID, userID uuid.UUID) error {
	return apperr.Wrap(s.ownerRepository.AddUser(ctx, uuid.Must(uuid.NewV7()), nodeID, userID))
}

func (s *PortsService) OwnedAmong(ctx context.Context, userID uuid.UUID, nodeIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	out, err := s.ownerRepository.OwnedAmong(ctx, userID, nodeIDs)
	return out, apperr.Wrap(err)
}

type PortsService struct {
	catalog         *catalogservices.CatalogServices
	ownerRepository *repositories.OwnerRepository
}
