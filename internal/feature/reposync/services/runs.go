package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	notifyservices "svc-registry/internal/feature/notify/services"
	notifytypes "svc-registry/internal/feature/notify/types"
	reposyncports "svc-registry/internal/feature/reposync/ports"
	"svc-registry/internal/feature/reposync/repositories"
	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
	"svc-registry/pkg/secretbox"
)

func (s *SyncService) StartSync(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID) error {
	if _, err := s.connectionsService.authorizeConnection(ctx, p, catalogtypes.PermWrite, nodeID, id); err != nil {
		return err
	}
	return apperr.Wrap(s.connectionRepository.ScheduleNow(ctx, id, reposynctypes.TriggerManual))
}

func (s *SyncService) ListRuns(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID) ([]reposynctypes.Run, error) {
	if _, err := s.connectionsService.authorizeConnection(ctx, p, catalogtypes.PermRead, nodeID, id); err != nil {
		return nil, err
	}
	runs, err := s.runRepository.List(ctx, id, int(s.cfg.Jobs.RepoSyncRunsKept))
	return runs, apperr.Wrap(err)
}

func (s *SyncService) finishRun(ctx context.Context, conn reposynctypes.Connection, run reposynctypes.Run, res reposynctypes.RunResult) error {
	return s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.runRepository.Finish(ctx, run.ID, res.Stored()); err != nil {
			return err
		}
		if err := s.runRepository.Prune(ctx, run.ConnectionID, int(s.cfg.Jobs.RepoSyncRunsKept)); err != nil {
			return err
		}
		ev := notifytypes.Event{Type: notifytypes.EventRepoSyncFailed, NodeID: conn.NodeID, DedupeKey: notifytypes.RepoSyncKey(run.ID),
			Episode: notifytypes.RepoSyncEpisode(conn.ID), OccurredAt: time.Now(),
			Data: notifytypes.RepoSyncData{ConnectionID: conn.ID, Hosting: string(conn.Kind), OwnerPath: conn.OwnerPath, RunID: run.ID,
				Trigger: string(run.Trigger), ErrorCode: res.ErrorCode, ErrorMessage: res.ErrorMessage}}
		switch res.Status {
		case reposynctypes.RunSucceeded:
			ev.Type = notifytypes.EventRepoSyncRecovered
			return s.notify.Outbox.Recover(ctx, ev)
		case reposynctypes.RunFailed:
			previous, err := s.runRepository.PreviousStatus(ctx, conn.ID, run.ID)
			if err != nil || (previous != nil && *previous == reposynctypes.RunFailed) {
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
	clients              reposyncports.ClientFactoryPort
	connectionRepository *repositories.ConnectionRepository
	db                   *pgtx.DB
	exclusionRepository  *repositories.ExclusionRepository
	notify               *notifyservices.NotifyServices
	repoRepository       *repositories.RepoRepository
	runRepository        *repositories.RunRepository
	secrets              *secretbox.Box
	connectionsService   *ConnectionsService
	unlink               *UnlinkService
	docs                 reposyncports.DocsTriggerPort
}
