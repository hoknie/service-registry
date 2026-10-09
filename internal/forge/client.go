package forge

import (
	"context"

	"svc-registry/internal/catalog"
)

type Client interface {
	Check(ctx context.Context) error
	ListRepos(ctx context.Context) ([]RemoteRepo, error)
	Details(ctx context.Context, repo RemoteRepo) (Details, error)
	ListBranches(ctx context.Context, repo RemoteRepo, keep func(name string) bool, limit int) (branches []catalog.ForgeBranch, truncated bool, err error)
	Repo(ctx context.Context, fullPath string) (RemoteRepo, error)
	ListTree(ctx context.Context, repo RemoteRepo, commit string) (entries []TreeEntry, truncated bool, err error)
	ReadFile(ctx context.Context, repo RemoteRepo, entry TreeEntry, limit int64) ([]byte, error)
	RegisterHook(ctx context.Context, url, secret string) (string, error)
	DeleteHook(ctx context.Context, id string) error
}

type Endpoint struct {
	Kind   Kind
	APIURL string
	Owner  string
	Token  string
}

type ClientFactory interface {
	New(endpoint Endpoint) (Client, error)
	VerifyDelivery(kind Kind, secret string, header func(name string) string, body []byte) bool
}

type TreeEntry struct {
	Path    string
	BlobSHA string
	Size    int64
}
