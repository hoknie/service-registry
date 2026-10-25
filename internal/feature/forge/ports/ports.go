package ports

import (
	"context"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"

	forgetypes "svc-registry/internal/feature/forge/types"
)

type ClientPort interface {
	Check(ctx context.Context) error
	ListRepos(ctx context.Context) ([]forgetypes.RemoteRepo, error)
	Details(ctx context.Context, repo forgetypes.RemoteRepo) (forgetypes.Details, error)
	ListBranches(ctx context.Context, repo forgetypes.RemoteRepo, keep func(name string) bool, limit int) (branches []catalogtypes.ForgeBranch, truncated bool, err error)
	Repo(ctx context.Context, fullPath string) (forgetypes.RemoteRepo, error)
	ListTree(ctx context.Context, repo forgetypes.RemoteRepo, commit string) (entries []forgetypes.TreeEntry, truncated bool, err error)
	ReadFile(ctx context.Context, repo forgetypes.RemoteRepo, entry forgetypes.TreeEntry, limit int64) ([]byte, error)
	RegisterHook(ctx context.Context, url, secret string) (string, error)
	DeleteHook(ctx context.Context, id string) error
}

type ClientFactoryPort interface {
	New(endpoint forgetypes.Endpoint) (ClientPort, error)
	VerifyDelivery(kind forgetypes.Kind, secret string, header func(name string) string, body []byte) bool
}

type ProjectSourcesPort interface {
	AdoptRemote(ctx context.Context, projectID uuid.UUID, src forgetypes.RemoteSource) error
	DropOwn(ctx context.Context, projectID uuid.UUID) error
}

const (
	EventForgeWebhook = "forge.webhook"
	EventForgeManual  = "forge.manual"
)

type DocsTriggerPort interface {
	TriggerCollection(ctx context.Context, projectID uuid.UUID, event string) error
}
