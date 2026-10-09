package catalog

import (
	"context"

	"github.com/google/uuid"
)

type NodeStore interface {
	Chain(ctx context.Context, userID, id uuid.UUID) (*NodeChain, error)
	Walk(ctx context.Context, walk Walk) ([]WalkNode, uint64, error)
	Table(ctx context.Context, walk Walk) ([]TableNode, uint64, error)
	Search(ctx context.Context, search Search) ([]TableNode, uint64, error)
	Get(ctx context.Context, id uuid.UUID) (*Node, error)
	ChildBySlug(ctx context.Context, parent *uuid.UUID, slug string) (*Node, error)
	Insert(ctx context.Context, node NewNode) (Node, error)
	Update(ctx context.Context, id uuid.UUID, changes NodeChanges) (Node, error)
	Move(ctx context.Context, id uuid.UUID, parent *uuid.UUID) (Node, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type BranchStore interface {
	List(ctx context.Context, projectID uuid.UUID, filter BranchFilter) ([]Branch, uint64, error)
	Get(ctx context.Context, projectID uuid.UUID, name string) (*Branch, error)
	SetDefault(ctx context.Context, projectID uuid.UUID, name *string) error
	SyncForge(ctx context.Context, projectID uuid.UUID, branches []ForgeBranch, complete bool) error
	SetPinned(ctx context.Context, projectID uuid.UUID, name string, pinned bool) (*Branch, error)
	Delete(ctx context.Context, projectID uuid.UUID, name string) error
	Prune(ctx context.Context, retentionDays, staleDays uint32) (int64, error)
}

type BindingStore interface {
	List(ctx context.Context, nodeIDs []uuid.UUID) ([]Binding, error)
	Put(ctx context.Context, binding NewBinding) (*Binding, error)
	Delete(ctx context.Context, nodeID, id uuid.UUID) error
}

type ProjectKeyStore interface {
	List(ctx context.Context, projectID uuid.UUID) ([]ProjectKey, error)
	Rotate(ctx context.Context, key NewKey, graceSecs uint64) (ProjectKey, error)
	Revoke(ctx context.Context, projectID, id uuid.UUID) error
	Verify(ctx context.Context, projectID uuid.UUID, hash [32]byte) (*uuid.UUID, error)
}
