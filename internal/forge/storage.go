package forge

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ConnectionStore interface {
	Insert(ctx context.Context, conn NewConnection) (Connection, error)
	List(ctx context.Context, nodeID uuid.UUID) ([]Connection, error)
	Find(ctx context.Context, id uuid.UUID) (*Connection, error)
	Secrets(ctx context.Context, id uuid.UUID) (*Secrets, error)
	Update(ctx context.Context, id uuid.UUID, settings Settings, creds *StoredCredentials) (Connection, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SetWebhook(ctx context.Context, id uuid.UUID, hook Webhook) error
	ScheduleNow(ctx context.Context, id uuid.UUID, trigger Trigger) error
	Claim(ctx context.Context, limit, leaseSecs int) ([]Claimed, error)
	ClaimOne(ctx context.Context, id uuid.UUID, leaseSecs int) (bool, error)
	Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error
	Release(ctx context.Context, id uuid.UUID, next *time.Time) error
	AllSecrets(ctx context.Context) ([]SecretRow, error)
	ReplaceSecrets(ctx context.Context, row SecretRow) error
}

type RepositoryStore interface {
	Links(ctx context.Context, connID uuid.UUID) ([]Link, error)
	Groups(ctx context.Context, connID uuid.UUID) (map[string]uuid.UUID, error)
	LinkGroup(ctx context.Context, connID, nodeID uuid.UUID, fullPath string) error
	Upsert(ctx context.Context, rec RepoRecord) error
	Orphan(ctx context.Context, projectIDs []uuid.UUID) error
	Find(ctx context.Context, projectID uuid.UUID) (*Repository, error)
	Managed(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error)
	Readme(ctx context.Context, projectID uuid.UUID) (*Readme, error)
}

type RunStore interface {
	Start(ctx context.Context, run NewRun) (Run, error)
	Finish(ctx context.Context, id uuid.UUID, result RunResult, keep int) error
	List(ctx context.Context, connID uuid.UUID, limit int) ([]Run, error)
}
