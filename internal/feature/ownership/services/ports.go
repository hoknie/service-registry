package services

import (
	"context"

	"github.com/google/uuid"

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
	_, out, err := s.ownerRepository.Effective(ctx, id)
	return out, apperr.Wrap(err)
}

type PortsService struct {
	ownerRepository *repositories.OwnerRepository
}
