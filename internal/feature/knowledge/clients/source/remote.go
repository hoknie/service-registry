package source

import (
	"context"
	"strings"
	"time"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	reposyncports "svc-registry/internal/feature/reposync/ports"
	reposynctypes "svc-registry/internal/feature/reposync/types"
)

type remote struct {
	client      reposyncports.ClientPort
	fullPath    string
	settings    knowledgetypes.Settings
	maxBranches int
	repo        *reposynctypes.RemoteRepo
}

func (r *remote) Heads(ctx context.Context) (map[string]string, string, error) {
	repo, err := r.client.Repo(ctx, r.fullPath)
	if err != nil {
		return nil, "", err
	}
	r.repo = &repo
	branches, _, err := r.client.ListBranches(ctx, repo, func(name string) bool {
		return r.settings.CollectsBranch(name, repo.DefaultBranch)
	}, r.maxBranches)
	if err != nil {
		return nil, "", err
	}
	heads := map[string]string{}
	for _, b := range branches {
		if b.HeadSHA != "" {
			heads[b.Name] = b.HeadSHA
		}
	}
	return heads, repo.DefaultBranch, nil
}

func (r *remote) Tags(ctx context.Context) (map[string]string, error) {
	repo, err := r.current(ctx)
	if err != nil {
		return nil, err
	}
	tags, _, err := r.client.ListTags(ctx, repo, r.settings.CollectsTag, r.maxBranches)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(tags))
	for _, t := range tags {
		if t.Commit != "" {
			out[t.Name] = t.Commit
		}
	}
	return out, nil
}

func (r *remote) current(ctx context.Context) (reposynctypes.RemoteRepo, error) {
	if r.repo == nil {
		if _, _, err := r.Heads(ctx); err != nil {
			return reposynctypes.RemoteRepo{}, err
		}
	}
	return *r.repo, nil
}

func (r *remote) Tree(ctx context.Context, _, head string) ([]knowledgetypes.Entry, bool, error) {
	repo, err := r.current(ctx)
	if err != nil {
		return nil, false, err
	}
	tree, truncated, err := r.client.ListTree(ctx, repo, head)
	if err != nil {
		return nil, false, err
	}
	out := make([]knowledgetypes.Entry, 0, len(tree))
	for _, e := range tree {
		out = append(out, knowledgetypes.Entry{Path: e.Path, BlobSHA: e.BlobSHA, Size: e.Size})
	}
	return out, truncated, nil
}

func (r *remote) Read(ctx context.Context, e knowledgetypes.Entry, limit int64) ([]byte, error) {
	repo, err := r.current(ctx)
	if err != nil {
		return nil, err
	}
	return r.client.ReadFile(ctx, repo, reposynctypes.TreeEntry{Path: e.Path, BlobSHA: e.BlobSHA, Size: e.Size}, limit)
}

func (r *remote) Metadata(ctx context.Context, known *time.Time) (reposynctypes.RemoteRepo, *reposynctypes.Details, error) {
	repo, err := r.current(ctx)
	if err != nil {
		return reposynctypes.RemoteRepo{}, nil, err
	}
	if changed := repo.ChangedAt(); known != nil && changed != nil && !changed.After(*known) {
		return repo, nil, nil
	}
	d, err := r.client.Details(ctx, repo)
	if err != nil {
		return repo, nil, err
	}
	return repo, &d, nil
}

func owner(fullPath string) string {
	o, _, _ := strings.Cut(fullPath, "/")
	return o
}
