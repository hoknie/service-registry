package ports

import (
	"context"

	"github.com/google/uuid"
)

const (
	EventServiceDeployed = "service.deployed"
	EventClusterDeployed = "cluster.deployed"
)

type DocsTriggerPort interface {
	TriggerCollection(ctx context.Context, projectID uuid.UUID, event string) error
}
