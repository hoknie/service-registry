package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/feature/ingest"
	"svc-registry/internal/feature/ingest/repository"
	"svc-registry/internal/platform/apperr"
)

type Authenticated struct {
	projectID uuid.UUID
	keyID     uuid.UUID
}

func (s *Service) AuthenticateIngest(ctx context.Context, projectID uuid.UUID, bearer *string) (Authenticated, error) {
	if bearer == nil {
		return Authenticated{}, apperr.New(apperr.InvalidProjectKey)
	}
	keyID, err := s.catalog.VerifyProjectKey(ctx, projectID, *bearer)
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

func (s *Service) AcceptEvent(ctx context.Context, who Authenticated, body ingest.WireEnvelope) (ingest.Accepted, error) {
	envelope, err := ingest.ParseEnvelope(body)
	if err != nil {
		return ingest.Accepted{}, apperr.Wrap(err)
	}
	var accepted ingest.Accepted
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		var err error
		accepted, err = s.events.Accept(ctx, ingest.NewEvent{
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

func (s *Service) recordBranch(ctx context.Context, projectID uuid.UUID, envelope ingest.Envelope) error {
	p := envelope.ServiceDeployed
	if p == nil || p.Branch == nil {
		return nil
	}
	name := strings.TrimPrefix(*p.Branch, "refs/heads/")
	if name == "" {
		return nil
	}
	return s.catalog.RecordIngestBranch(ctx, projectID, name, p.CommitSHA, envelope.OccurredAt)
}

func (s *Service) RecordDeployment(ctx context.Context, d ingest.DeploymentRecord) (bool, error) {
	return repository.InsertDeployment(ctx, s.db.From(ctx), d)
}

func (s *Service) CurrentEnvironments(ctx context.Context, projectID uuid.UUID) ([]ingest.Environment, error) {
	return s.deployments.Environments(ctx, projectID)
}

func (s *Service) PruneEvents(ctx context.Context) (uint64, error) {
	n, err := s.events.Prune(ctx, s.cfg.Ingest.RetentionDays)
	return n, apperr.Wrap(err)
}
