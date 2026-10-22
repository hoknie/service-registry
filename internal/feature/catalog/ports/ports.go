package ports

import (
	"context"

	catalogtypes "svc-registry/internal/feature/catalog/types"

	"github.com/google/uuid"
)

type ActivitySourcePort interface {
	ActivitySignals(ctx context.Context, projects []uuid.UUID, pending, busy bool) (map[uuid.UUID][]catalogtypes.ProcessSignals, error)
}

type ContainerActivitySourcePort interface {
	ContainerSignals(ctx context.Context, nodeID uuid.UUID) ([]catalogtypes.ProcessSignals, error)
}

type ManagedNodesPort interface {
	Managed(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error)
}

type ProjectSourcePort interface {
	Repo() (*catalogtypes.Forge, *string, bool)
	Attach(ctx context.Context, projectID uuid.UUID) error
}

type SourceFactoryPort func(projectID uuid.UUID) (ProjectSourcePort, error)

type SecretUserPort interface {
	SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error)
}
