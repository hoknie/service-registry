package ingest

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/access"
)

type EventStore interface {
	Accept(ctx context.Context, event NewEvent) (Accepted, error)
	List(ctx context.Context, projectID uuid.UUID, filter EventFilter, page access.PageRequest) (access.Page[Event], error)
	Prune(ctx context.Context, retentionDays uint32) (uint64, error)
}

type DeploymentStore interface {
	List(ctx context.Context, projectID uuid.UUID, filter DeploymentFilter, page access.PageRequest) (access.Page[Deployment], error)
	Environments(ctx context.Context, projectID uuid.UUID) ([]Environment, error)
}
