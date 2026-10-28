package ports

import (
	"context"
	"io"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"

	reposynctypes "svc-registry/internal/feature/reposync/types"
)

type ClientPort interface {
	Check(ctx context.Context) error
	ListRepos(ctx context.Context) ([]reposynctypes.RemoteRepo, error)
	Details(ctx context.Context, repo reposynctypes.RemoteRepo) (reposynctypes.Details, error)
	ListBranches(ctx context.Context, repo reposynctypes.RemoteRepo, keep func(name string) bool, limit int) (branches []catalogtypes.RepoSyncBranch, truncated bool, err error)
	ListTags(ctx context.Context, repo reposynctypes.RemoteRepo, keep func(name string) bool, limit int) (tags []reposynctypes.Tag, truncated bool, err error)
	Repo(ctx context.Context, fullPath string) (reposynctypes.RemoteRepo, error)
	ListTree(ctx context.Context, repo reposynctypes.RemoteRepo, commit string) (entries []reposynctypes.TreeEntry, truncated bool, err error)
	ReadFile(ctx context.Context, repo reposynctypes.RemoteRepo, entry reposynctypes.TreeEntry, limit int64) ([]byte, error)
	Archive(ctx context.Context, repo reposynctypes.RemoteRepo, commit string) (io.ReadCloser, error)
	RegisterHook(ctx context.Context, url, secret string) (string, error)
	DeleteHook(ctx context.Context, id string) error
}

type ClientFactoryPort interface {
	New(endpoint reposynctypes.Endpoint) (ClientPort, error)
	VerifyDelivery(kind reposynctypes.Kind, secret string, header func(name string) string, body []byte) bool
}

type ProjectSourcesPort interface {
	AdoptRemote(ctx context.Context, projectID uuid.UUID, src reposynctypes.RemoteSource) error
	DropOwn(ctx context.Context, projectID uuid.UUID) error
}

const (
	EventRepoSyncWebhook = "repo_sync.webhook"
	EventRepoSyncManual  = "repo_sync.manual"
)

type DocsTriggerPort interface {
	TriggerCollection(ctx context.Context, projectID uuid.UUID, event string) error
}
