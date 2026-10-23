package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	forgeports "svc-registry/internal/feature/forge/ports"
	"svc-registry/internal/feature/forge/repositories"
	forgetypes "svc-registry/internal/feature/forge/types"
	notifyservices "svc-registry/internal/feature/notify/services"
	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
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

func (s *SyncService) finishRun(ctx context.Context, conn forgetypes.Connection, run forgetypes.Run, res forgetypes.RunResult) error {
	return s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.runRepository.Finish(ctx, run.ID, res, int(s.cfg.Jobs.ForgeRunsKept)); err != nil {
			return err
		}
		ev := notifytypes.Event{Type: notifytypes.EventForgeSyncFailed, NodeID: conn.NodeID, DedupeKey: notifytypes.ForgeKey(run.ID),
			Episode: notifytypes.ForgeEpisode(conn.ID), OccurredAt: time.Now(),
			Data: notifytypes.ForgeData{ConnectionID: conn.ID, Forge: string(conn.Kind), OwnerPath: conn.OwnerPath, RunID: run.ID,
				Trigger: string(run.Trigger), ErrorCode: res.ErrorCode, ErrorMessage: res.ErrorMessage}}
		switch res.Status {
		case forgetypes.RunSucceeded:
			ev.Type = notifytypes.EventForgeRecovered
			return s.notify.Outbox.Recover(ctx, ev)
		case forgetypes.RunFailed:
			previous, err := s.runRepository.PreviousStatus(ctx, conn.ID, run.ID)
			if err != nil || (previous != nil && *previous == forgetypes.RunFailed) {
				return err
			}
			return s.notify.Outbox.Emit(ctx, ev)
		}
		return nil
	})
}

type SyncService struct {
	adoptionRepository   *repositories.AdoptionRepository
	catalog              *catalogservices.CatalogServices
	cfg                  *config.Config
	clients              forgeports.ClientFactoryPort
	connectionRepository *repositories.ConnectionRepository
	db                   *pgtx.DB
	exclusionRepository  *repositories.ExclusionRepository
	notify               *notifyservices.NotifyServices
	repoRepository       *repositories.RepoRepository
	runRepository        *repositories.RunRepository
	secrets              *secretbox.Box
	connectionsService   *ConnectionsService
	unlink               *UnlinkService
	docs                 forgeports.DocsTriggerPort
}
