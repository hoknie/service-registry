package ports

import (
	"context"

	accesstypes "svc-registry/internal/feature/access/types"

	"github.com/google/uuid"
)

type NodeRootsPort interface {
	CheckRoots(ctx context.Context, owner accesstypes.Principal, ids []uuid.UUID) (bool, error)
	DescribeRoots(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]accesstypes.NodeRef, error)
}

type ClientMetadataPort interface {
	Fetch(ctx context.Context, url string) (accesstypes.FetchedMetadata, error)
}
