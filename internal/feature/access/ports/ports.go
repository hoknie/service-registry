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

type GroupStewardsPort interface {
	GroupNodes(ctx context.Context, groupIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)
	OwnedAmong(ctx context.Context, userID uuid.UUID, nodeIDs []uuid.UUID) ([]uuid.UUID, error)
}

type MCPServersPort interface {
	VisibleServers(ctx context.Context, owner accesstypes.Principal, ids []uuid.UUID) (bool, error)
	DescribeServers(ctx context.Context, ids []uuid.UUID) ([]accesstypes.ServerRef, error)
	ListServers(ctx context.Context, owner accesstypes.Principal) ([]accesstypes.ServerRef, error)
}

type OrganizationsPort interface {
	OrganizationAccess(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID) (accesstypes.OrganizationAccess, error)
}

type InvitationsPort interface {
	PendingInvitation(ctx context.Context, hash [32]byte) (string, bool, error)
	AcceptInvitation(ctx context.Context, hash [32]byte, user uuid.UUID, email string) error
}
