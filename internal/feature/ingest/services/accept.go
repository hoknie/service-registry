package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	"svc-registry/internal/feature/ingest/repositories"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
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
	var accepted ingesttypes.Accepted
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		var err error
		accepted, err = s.eventRepository.Accept(ctx, ingesttypes.NewEvent{
			ID: uuid.Must(uuid.NewV7()), ProjectID: who.projectID, KeyID: who.keyID,
			DeploymentID: uuid.Must(uuid.NewV7()), EnvironmentID: uuid.Must(uuid.NewV7()),
			Envelope: envelope,
		})
		if err != nil || accepted.Replayed {
			return err
		}
		return s.recordBranch(ctx, who.projectID, envelope)
	})
	return accepted, apperr.Wrap(err)
}

func (s *AcceptService) recordBranch(ctx context.Context, projectID uuid.UUID, envelope ingesttypes.Envelope) error {
	p := envelope.ServiceDeployed
	if p == nil || p.Branch == nil {
		return nil
	}
	name := strings.TrimPrefix(*p.Branch, "refs/heads/")
	if name == "" {
		return nil
	}
	return s.catalog.BranchRecords.RecordIngestBranch(ctx, projectID, name, p.CommitSHA, envelope.OccurredAt)
}

func (s *AcceptService) RecordDeployment(ctx context.Context, d ingesttypes.DeploymentRecord) (bool, error) {
	return repositories.InsertDeployment(ctx, s.db.From(ctx), d)
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
	db                   *postgres.DB
	deploymentRepository *repositories.DeploymentRepository
	eventRepository      *repositories.EventRepository
	limiter              *auth.IngestLimiter
}
