package deploy

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/forge"
)

type EnvironmentStore interface {
	List(ctx context.Context) ([]Environment, error)
	Insert(ctx context.Context, e NewEnvironment) (Environment, error)
	Update(ctx context.Context, key string, c EnvironmentChanges) (*Environment, error)
	Delete(ctx context.Context, key string) error
}

type StoredSecret struct {
	ID  uuid.UUID
	Enc string
}

type ClusterStore interface {
	List(ctx context.Context) ([]Cluster, error)
	Get(ctx context.Context, id uuid.UUID) (*Cluster, error)
	Insert(ctx context.Context, c NewCluster) (Cluster, error)
	Update(ctx context.Context, id uuid.UUID, u ClusterUpdate) (*Cluster, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Stored(ctx context.Context, id uuid.UUID) (*forge.StoredCredentials, error)
	ScheduleNow(ctx context.Context, id uuid.UUID) error
	Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error)
	Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error
	Finish(ctx context.Context, id uuid.UUID, failure *Failure) error
	Secrets(ctx context.Context) ([]StoredSecret, error)
	ReplaceSecret(ctx context.Context, id uuid.UUID, enc string) error
}

type ProjectRef struct {
	ID      uuid.UUID
	Observe bool
}

type WorkloadStore interface {
	Previous(ctx context.Context, clusterID uuid.UUID) (map[string]Previous, error)
	Save(ctx context.Context, clusterID uuid.UUID, obs []Observation, deployments []ClusterDeployment) error
	Unmatched(ctx context.Context, clusterID uuid.UUID, limit uint32, offset uint64) ([]Unmatched, uint64, error)
	ForProject(ctx context.Context, projectID uuid.UUID) ([]Observed, error)
	Prune(ctx context.Context, days uint32) (int64, error)
	Project(ctx context.Context, ref string) (*ProjectRef, error)
	Current(ctx context.Context, projectID uuid.UUID, service, environment string) (*string, error)
}
