package links

import (
	"context"

	"github.com/google/uuid"
)

type KindStore interface {
	List(ctx context.Context) ([]Kind, error)
	Insert(ctx context.Context, kind NewKind) (Kind, error)
	Update(ctx context.Context, key string, changes KindChanges) (*Kind, error)
	Delete(ctx context.Context, key string) error
}

type TemplateStore interface {
	Effective(ctx context.Context, nodeID uuid.UUID) ([]Template, error)
	Put(ctx context.Context, t NewTemplate) (Template, error)
	Delete(ctx context.Context, nodeID uuid.UUID, linkKey string) error
	EffectiveVars(ctx context.Context, nodeID uuid.UUID) ([]Var, error)
	ReplaceVars(ctx context.Context, nodeID uuid.UUID, vars map[string]string) error
	ProjectData(ctx context.Context, projectID uuid.UUID) (ProjectData, error)
	IsUnder(ctx context.Context, node, root uuid.UUID) (bool, error)
	Projects(ctx context.Context, after uuid.UUID, limit int) ([]uuid.UUID, error)
}

type ProjectData struct {
	Templates    []Template
	Vars         []Var
	RepoFullPath *string
	Deployments  []Deployment
}

type Target struct {
	ID  uuid.UUID
	URL string
}

type TargetStore interface {
	Seen(ctx context.Context, urls []string) error
	Latest(ctx context.Context, urls []string) (map[string]Check, error)
	Fresh(ctx context.Context, url string, withinSecs int) (*Check, error)
	Record(ctx context.Context, url string, r Result, intervalSecs, history uint32) (Check, error)
	History(ctx context.Context, urls []string) ([]Check, error)
	Claim(ctx context.Context, limit, leaseSecs int) ([]Target, error)
	Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error
	Prune(ctx context.Context, ttlDays uint32) (int64, error)
}

type Checker interface {
	Check(ctx context.Context, url string) Result
}
