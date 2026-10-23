package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	"svc-registry/internal/feature/ingest/repositories"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	notifyservices "svc-registry/internal/feature/notify/services"
	notifytypes "svc-registry/internal/feature/notify/types"
	topologyservices "svc-registry/internal/feature/topology/services"
	topologytypes "svc-registry/internal/feature/topology/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
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
		if err := s.recordBranch(ctx, who.projectID, envelope); err != nil {
			return err
		}
		if err := s.recordDependencies(ctx, who.projectID, envelope, accepted.Deployment); err != nil {
			return err
		}
		return s.emitDeployed(ctx, who.projectID, envelope, accepted.Deployment)
	})
	return accepted, apperr.Wrap(err)
}

func (s *AcceptService) emitDeployed(ctx context.Context, projectID uuid.UUID, envelope ingesttypes.Envelope, d *ingesttypes.DeploymentResult) error {
	p := envelope.ServiceDeployed
	if p == nil || d == nil {
		return nil
	}
	at, err := time.Parse(time.RFC3339, envelope.OccurredAt)
	if err != nil {
		at = time.Now()
	}
	return s.emit(ctx, projectID, notifytypes.DeploymentData{DeploymentID: d.ID, Service: p.Service, Environment: p.Environment,
		Version: p.Version, CommitSHA: p.CommitSHA, Branch: p.Branch, URL: p.URL, DeployedBy: p.DeployedBy, Source: "event",
		Cluster: p.Cluster, Namespace: p.Namespace, Current: d.BecameCurrent}, at)
}

func (s *AcceptService) emit(ctx context.Context, projectID uuid.UUID, d notifytypes.DeploymentData, at time.Time) error {
	return s.notify.Outbox.Emit(ctx, notifytypes.Event{Type: notifytypes.EventDeploymentCreated, NodeID: projectID,
		Environment: &d.Environment, DedupeKey: notifytypes.DeploymentKey(d.DeploymentID), OccurredAt: at, Data: d})
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

func (s *AcceptService) recordDependencies(ctx context.Context, projectID uuid.UUID, envelope ingesttypes.Envelope, d *ingesttypes.DeploymentResult) error {
	p := envelope.ServiceDeployed
	if p == nil || p.DependsOn == nil || d == nil || !d.BecameCurrent {
		return nil
	}
	return s.topology.Records.ReplaceOrigin(ctx, projectID, topologytypes.SourceEvent, p.Service+"/"+p.Environment, *p.DependsOn)
}

func (s *AcceptService) RecordDeployment(ctx context.Context, d ingesttypes.DeploymentRecord, at time.Time) (bool, error) {
	current, err := repositories.InsertDeployment(ctx, s.db.From(ctx), d)
	if err != nil {
		return false, err
	}
	return current, s.emit(ctx, d.ProjectID, notifytypes.DeploymentData{DeploymentID: d.ID, Service: d.Service,
		Environment: d.Environment, Version: d.Version, CommitSHA: d.CommitSHA, Branch: d.Branch, URL: d.URL,
		DeployedBy: d.DeployedBy, Source: d.Source, Cluster: d.Cluster, Namespace: d.Namespace, Current: current}, at)
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
	db                   *pgtx.DB
	deploymentRepository *repositories.DeploymentRepository
	eventRepository      *repositories.EventRepository
	limiter              *auth.IngestLimiter
	notify               *notifyservices.NotifyServices
	topology             *topologyservices.TopologyServices
}
