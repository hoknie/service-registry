package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	"svc-registry/internal/feature/ingest/repositories"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	"svc-registry/internal/feature/ingest/usecases"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
)

type Authenticated struct {
	projectID uuid.UUID
	keyID     uuid.UUID
}

func (s *AcceptService) AuthenticateIngest(ctx context.Context, projectID uuid.UUID, bearer *string) (Authenticated, error) {
	if bearer == nil {
		return Authenticated{}, apperr.New(apperr.InvalidProjectKey)
	}
	keyID, err := s.catalog.ProjectKeys.VerifyProjectKey(ctx, projectID, *bearer)
	if err != nil {
		return Authenticated{}, apperr.Wrap(err)
	}
	if keyID == nil {
		return Authenticated{}, apperr.New(apperr.InvalidProjectKey)
	}
	if secs, ok := s.limiter.Hit(*keyID, time.Now()); !ok {
		return Authenticated{}, &apperr.Error{Kind: apperr.IngestRateLimited, RetryAfterSecs: secs}
	}
	return Authenticated{projectID: projectID, keyID: *keyID}, nil
}

func (s *AcceptService) AcceptEvent(ctx context.Context, who Authenticated, body ingesttypes.WireEnvelope) (ingesttypes.Accepted, error) {
	envelope, err := ingesttypes.ParseEnvelope(body)
	if err != nil {
		return ingesttypes.Accepted{}, apperr.Wrap(err)
	}
	accepted, err := s.acceptEvent.Run(ctx, usecases.AcceptEventInput{ProjectID: who.projectID, KeyID: who.keyID, Envelope: envelope})
	return accepted, apperr.Wrap(err)
}

func (s *AcceptService) RecordDeployment(ctx context.Context, d ingesttypes.DeploymentRecord, at time.Time) (bool, error) {
	return s.recordDeployment.Run(ctx, d, at)
}

func (s *AcceptService) CurrentEnvironments(ctx context.Context, projectID uuid.UUID) ([]ingesttypes.Environment, error) {
	return s.deploymentRepository.Environments(ctx, projectID)
}

func (s *AcceptService) PruneEvents(ctx context.Context) (uint64, error) {
	n, err := s.eventRepository.Prune(ctx, s.cfg.Ingest.RetentionDays)
	return n, apperr.Wrap(err)
}

type AcceptService struct {
	catalog              *catalogservices.CatalogServices
	cfg                  *config.Config
	deploymentRepository *repositories.DeploymentRepository
	eventRepository      *repositories.EventRepository
	limiter              *auth.IngestLimiter
	acceptEvent          *usecases.AcceptEventUseCase
	recordDeployment     *usecases.RecordDeploymentUseCase
}
