package ports

import (
	"context"

	"github.com/google/uuid"
)

type DocsTriggerPort interface {
	TriggerCollection(ctx context.Context, projectID uuid.UUID) error
}
