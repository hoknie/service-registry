package usecases

import (
	"context"
	"time"

	"github.com/google/uuid"

	ingestports "svc-registry/internal/feature/ingest/ports"
	"svc-registry/internal/feature/ingest/repositories"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	notifyservices "svc-registry/internal/feature/notify/services"
	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/pkg/pgtx"
)

type RecordDeploymentUseCase struct {
	db                   *pgtx.DB
	deploymentRepository *repositories.DeploymentRepository
	notify               *notifyservices.NotifyServices
	docs                 ingestports.DocsTriggerPort
}

func NewRecordDeployment(db *pgtx.DB, deployments *repositories.DeploymentRepository, notify *notifyservices.NotifyServices) *RecordDeploymentUseCase {
	return &RecordDeploymentUseCase{db: db, deploymentRepository: deployments, notify: notify}
}

func (u *RecordDeploymentUseCase) Use(docs ingestports.DocsTriggerPort) { u.docs = docs }

func (u *RecordDeploymentUseCase) Run(ctx context.Context, d ingesttypes.DeploymentRecord, at time.Time) (bool, error) {
	if d.EnvironmentID == uuid.Nil {
		d.EnvironmentID = uuid.Must(uuid.NewV7())
	}
	d = d.Stored()
	var current bool
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		if err := u.deploymentRepository.Insert(ctx, d); err != nil {
			return err
		}
		advanced, err := u.deploymentRepository.AdvanceEnvironment(ctx, d)
		if err != nil {
			return err
		}
		if advanced {
			if err := u.deploymentRepository.MarkCurrent(ctx, d.ID); err != nil {
				return err
			}
		}
		current = advanced
		if d.Source == "cluster" && u.docs != nil {
			if err := u.docs.TriggerCollection(ctx, d.ProjectID, ingestports.EventClusterDeployed); err != nil {
				return err
			}
		}
		return u.emit(ctx, d, current, at)
	})
	return current, err
}

func (u *RecordDeploymentUseCase) emit(ctx context.Context, d ingesttypes.DeploymentRecord, current bool, at time.Time) error {
	data := notifytypes.DeploymentData{DeploymentID: d.ID, Service: d.Service, Environment: d.Environment, Version: d.Version,
		CommitSHA: d.CommitSHA, Branch: d.Branch, URL: d.URL, DeployedBy: d.DeployedBy, Source: d.Source, Cluster: d.Cluster,
		Namespace: d.Namespace, Current: current}
	return u.notify.Outbox.Emit(ctx, notifytypes.Event{Type: notifytypes.EventDeploymentCreated, NodeID: d.ProjectID,
		Environment: &data.Environment, DedupeKey: notifytypes.DeploymentKey(d.ID), OccurredAt: at, Data: data})
}
