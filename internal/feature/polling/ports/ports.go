package ports

import (
	"context"

	"github.com/google/uuid"
)

type ReplanPort interface {
	ReplanSubtree(ctx context.Context, nodeID uuid.UUID) error
}
