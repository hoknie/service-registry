package services

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/polling/repositories"
	pollingtypes "svc-registry/internal/feature/polling/types"
	"svc-registry/internal/platform/apperr"
)

type PortsService struct {
	pollingRepository *repositories.PollingRepository
	defaults          pollingtypes.Defaults
}

func (s *PortsService) Defaults() pollingtypes.Defaults { return s.defaults }

func (s *PortsService) Effective(ctx context.Context, nodeID uuid.UUID) (pollingtypes.Effective, error) {
	chain, err := s.pollingRepository.Chain(ctx, nodeID)
	if err != nil {
		return pollingtypes.Effective{}, apperr.Wrap(err)
	}
	e, _ := pollingtypes.Merge(chain, s.defaults.TimeoutSecs)
	return e, nil
}
