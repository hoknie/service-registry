package catalog

import (
	"context"

	"github.com/google/uuid"
)

type ActivitySource interface {
	ActivitySignals(ctx context.Context, projects []uuid.UUID, pending, busy bool) (map[uuid.UUID][]ProcessSignals, error)
}

type ManagedNodes interface {
	Managed(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error)
}

type ProjectSource interface {
	Repo() (*Forge, *string, bool)
	Attach(ctx context.Context, projectID uuid.UUID) error
}

type SourceFactory func(projectID uuid.UUID) (ProjectSource, error)
