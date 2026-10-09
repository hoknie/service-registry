package docsource

import (
	"context"
	"strings"

	"svc-registry/internal/forge"
	"svc-registry/internal/knowledge"
)

type remote struct {
	client      forge.Client
	fullPath    string
	settings    knowledge.Settings
	maxBranches int
	repo        *forge.RemoteRepo
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

func (r *remote) current(ctx context.Context) (forge.RemoteRepo, error) {
	if r.repo == nil {
		if _, _, err := r.Heads(ctx); err != nil {
			return forge.RemoteRepo{}, err
		}
	}
	return *r.repo, nil
}

func (r *remote) Tree(ctx context.Context, _, head string) ([]knowledge.Entry, bool, error) {
	repo, err := r.current(ctx)
	if err != nil {
		return nil, false, err
	}
	tree, truncated, err := r.client.ListTree(ctx, repo, head)
	if err != nil {
		return nil, false, err
	}
	out := make([]knowledge.Entry, 0, len(tree))
	for _, e := range tree {
		out = append(out, knowledge.Entry{Path: e.Path, BlobSHA: e.BlobSHA, Size: e.Size})
	}
	return out, truncated, nil
}

func (r *remote) Read(ctx context.Context, e knowledge.Entry, limit int64) ([]byte, error) {
	repo, err := r.current(ctx)
	if err != nil {
		return nil, err
	}
	return r.client.ReadFile(ctx, repo, forge.TreeEntry{Path: e.Path, BlobSHA: e.BlobSHA, Size: e.Size}, limit)
}

func owner(fullPath string) string {
	o, _, _ := strings.Cut(fullPath, "/")
	return o
}
