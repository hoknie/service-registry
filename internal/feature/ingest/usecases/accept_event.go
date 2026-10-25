package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	ingestports "svc-registry/internal/feature/ingest/ports"
	"svc-registry/internal/feature/ingest/repositories"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	topologyservices "svc-registry/internal/feature/topology/services"
	topologytypes "svc-registry/internal/feature/topology/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
)

type AcceptEventInput struct {
	ProjectID uuid.UUID
	KeyID     uuid.UUID
	Envelope  ingesttypes.Envelope
}

type AcceptEventUseCase struct {
	db                   *pgtx.DB
	eventRepository      *repositories.EventRepository
	deploymentRepository *repositories.DeploymentRepository
	recordDeployment     *RecordDeploymentUseCase
	catalog              *catalogservices.CatalogServices
	topology             *topologyservices.TopologyServices
	docs                 ingestports.DocsTriggerPort
}

type AcceptEventDeps struct {
	DB               *pgtx.DB
	Events           *repositories.EventRepository
	Deployments      *repositories.DeploymentRepository
	RecordDeployment *RecordDeploymentUseCase
	Catalog          *catalogservices.CatalogServices
	Topology         *topologyservices.TopologyServices
}

func NewAcceptEvent(d AcceptEventDeps) *AcceptEventUseCase {
	return &AcceptEventUseCase{db: d.DB, eventRepository: d.Events, deploymentRepository: d.Deployments,
		recordDeployment: d.RecordDeployment, catalog: d.Catalog, topology: d.Topology}
}

func (u *AcceptEventUseCase) Use(docs ingestports.DocsTriggerPort) { u.docs = docs }

func (u *AcceptEventUseCase) Run(ctx context.Context, in AcceptEventInput) (ingesttypes.Accepted, error) {
	envelope := in.Envelope
	if err := ingesttypes.CheckOccurredAt(envelope.OccurredAt, time.Now()); err != nil {
		return ingesttypes.Accepted{}, err
	}
	e := ingesttypes.NewEvent{
		ID: uuid.Must(uuid.NewV7()), ProjectID: in.ProjectID, KeyID: in.KeyID,
		DeploymentID: uuid.Must(uuid.NewV7()), EnvironmentID: uuid.Must(uuid.NewV7()), Envelope: envelope,
	}
	var accepted ingesttypes.Accepted
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		inserted, err := u.eventRepository.Insert(ctx, e)
		if err != nil {
			return err
		}
		if !inserted {
			accepted, err = u.replay(ctx, e)
			return err
		}
		if accepted, err = u.accept(ctx, e); err != nil {
			return err
		}
		return u.followUp(ctx, e, accepted.Deployment)
	})
	return accepted, err
}

func (u *AcceptEventUseCase) replay(ctx context.Context, e ingesttypes.NewEvent) (ingesttypes.Accepted, error) {
	replay, err := u.eventRepository.FindReplay(ctx, e)
	if err != nil {
		return ingesttypes.Accepted{}, err
	}
	if replay == nil {
		return ingesttypes.Accepted{}, apperr.InternalDetail("idempotent event vanished during replay")
	}
	if !replay.Same {
		return ingesttypes.Accepted{}, ingesttypes.ConflictIdempotencyKeyReused
	}
	event, err := u.eventRepository.Load(ctx, replay.EventID)
	if err != nil {
		return ingesttypes.Accepted{}, err
	}
	deployment, err := u.deploymentRepository.ByEvent(ctx, replay.EventID)
	if err != nil {
		return ingesttypes.Accepted{}, err
	}
	return ingesttypes.Accepted{Event: event, Replayed: true, Deployment: deployment}, nil
}

func (u *AcceptEventUseCase) accept(ctx context.Context, e ingesttypes.NewEvent) (ingesttypes.Accepted, error) {
	var deployment *ingesttypes.DeploymentResult
	if p := e.Envelope.ServiceDeployed; p != nil {
		record := e.Deployment(p)
		current, err := u.recordDeployment.Run(ctx, record, occurredAt(e.Envelope))
		if err != nil {
			return ingesttypes.Accepted{}, err
		}
		deployment = new(record.Result(current))
	}
	event, err := u.eventRepository.Load(ctx, e.ID)
	if err != nil {
		return ingesttypes.Accepted{}, err
	}
	return ingesttypes.Accepted{Event: event, Deployment: deployment}, nil
}

func (u *AcceptEventUseCase) followUp(ctx context.Context, e ingesttypes.NewEvent, d *ingesttypes.DeploymentResult) error {
	if err := u.recordBranch(ctx, e); err != nil {
		return err
	}
	if u.docs != nil {
		if err := u.docs.TriggerCollection(ctx, e.ProjectID, ingestports.EventServiceDeployed); err != nil {
			return err
		}
	}
	p := e.Envelope.ServiceDeployed
	if p == nil || p.DependsOn == nil || d == nil || !d.BecameCurrent {
		return nil
	}
	return u.topology.Records.ReplaceOrigin(ctx, e.ProjectID, topologytypes.SourceEvent, p.Service+"/"+p.Environment, *p.DependsOn)
}

func (u *AcceptEventUseCase) recordBranch(ctx context.Context, e ingesttypes.NewEvent) error {
	p := e.Envelope.ServiceDeployed
	if p == nil || p.Branch == nil {
		return nil
	}
	name := strings.TrimPrefix(*p.Branch, "refs/heads/")
	if name == "" {
		return nil
	}
	return u.catalog.BranchRecords.RecordIngestBranch(ctx, e.ProjectID, name, p.CommitSHA, e.Envelope.OccurredAt)
}

func occurredAt(envelope ingesttypes.Envelope) time.Time {
	at, err := time.Parse(time.RFC3339, envelope.OccurredAt)
	if err != nil {
		return time.Now()
	}
	return at
}
