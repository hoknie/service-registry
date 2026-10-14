package forge

import (
	"context"

	"github.com/google/uuid"
)

type ProjectSources interface {
	AdoptRemote(ctx context.Context, projectID uuid.UUID, src RemoteSource) error
	DropOwn(ctx context.Context, projectID uuid.UUID) error
}
