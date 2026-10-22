package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	forgeports "svc-registry/internal/feature/forge/ports"
	"svc-registry/internal/feature/forge/repositories"
	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/secretbox"
)

func (s *SyncService) StartSync(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID) error {
	if _, err := s.connectionsService.authorizeConnection(ctx, p, catalogtypes.PermWrite, nodeID, id); err != nil {
		return err
	}
	return apperr.Wrap(s.connectionRepository.ScheduleNow(ctx, id, forgetypes.TriggerManual))
}

func (s *SyncService) ListRuns(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID) ([]forgetypes.Run, error) {
	if _, err := s.connectionsService.authorizeConnection(ctx, p, catalogtypes.PermRead, nodeID, id); err != nil {
		return nil, err
	}
	runs, err := s.runRepository.List(ctx, id, int(s.cfg.Jobs.ForgeRunsKept))
	return runs, apperr.Wrap(err)
}

type SyncService struct {
	adoptionRepository   *repositories.AdoptionRepository
	catalog              *catalogservices.CatalogServices
	cfg                  *config.Config
	clients              forgeports.ClientFactoryPort
	connectionRepository *repositories.ConnectionRepository
	exclusionRepository  *repositories.ExclusionRepository
	repoRepository       *repositories.RepoRepository
	runRepository        *repositories.RunRepository
	secrets              *secretbox.Box
	connectionsService   *ConnectionsService
	unlink               *UnlinkService
}
