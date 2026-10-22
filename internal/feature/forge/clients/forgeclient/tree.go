package forgeclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"

	"code.gitea.io/sdk/gitea"
	gitlab "gitlab.com/gitlab-org/api/client-go"

	forgetypes "svc-registry/internal/feature/forge/types"
)

func cut(b []byte, limit int64) []byte {
	if int64(len(b)) > limit+1 {
		return b[:limit+1]
	}
	return b
}

func (g *githubClient) ListTree(ctx context.Context, repo forgetypes.RemoteRepo, commit string) ([]forgetypes.TreeEntry, bool, error) {
	owner, name := splitFull(repo.FullPath)
	tree, _, err := g.c.Git.GetTree(ctx, owner, name, commit, true)
	if err != nil {
		if isStatus(err, http.StatusNotFound) || isStatus(err, http.StatusConflict) || isStatus(err, http.StatusUnprocessableEntity) {
			return nil, false, forgetypes.ErrRepoNotFound
		}
		return nil, false, githubErr(err, false)
	}
	var out []forgetypes.TreeEntry
	for _, e := range tree.Entries {
		if e.GetType() != "blob" || e.GetMode() == "120000" {
			continue
		}
		size := int64(-1)
		if e.Size != nil {
			size = int64(*e.Size)
		}
		out = append(out, forgetypes.TreeEntry{Path: e.GetPath(), BlobSHA: e.GetSHA(), Size: size})
	}
	return out, tree.GetTruncated(), nil
}

func (g *githubClient) ReadFile(ctx context.Context, repo forgetypes.RemoteRepo, e forgetypes.TreeEntry, limit int64) ([]byte, error) {
	owner, name := splitFull(repo.FullPath)
	raw, _, err := g.c.Git.GetBlobRaw(ctx, owner, name, e.BlobSHA)
	if err != nil {
		if isStatus(err, http.StatusNotFound) {
			return nil, forgetypes.ErrRepoNotFound
		}
		return nil, githubErr(err, false)
	}
	return cut(raw, limit), nil
}

func (g *gitlabClient) ListTree(ctx context.Context, repo forgetypes.RemoteRepo, commit string) ([]forgetypes.TreeEntry, bool, error) {
	var out []forgetypes.TreeEntry
	opt := &gitlab.ListTreeOptions{Ref: new(commit), Recursive: new(true), ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1}}
	for {
		nodes, resp, err := g.c.Repositories.ListTree(repo.ExternalID, opt, gitlab.WithContext(ctx))
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return nil, false, forgetypes.ErrRepoNotFound
			}
			return nil, false, gitlabErr(err, resp, false)
		}
		for _, n := range nodes {
			if n.Type == "blob" && n.Mode != "120000" {
				out = append(out, forgetypes.TreeEntry{Path: n.Path, BlobSHA: n.ID, Size: -1})
			}
		}
		if resp.NextPage == 0 {
			return out, false, nil
		}
		opt.Page = resp.NextPage
	}
}

func (g *gitlabClient) ReadFile(ctx context.Context, repo forgetypes.RemoteRepo, e forgetypes.TreeEntry, limit int64) ([]byte, error) {
	raw, resp, err := g.c.Repositories.RawBlobContent(repo.ExternalID, e.BlobSHA, gitlab.WithContext(ctx))
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, forgetypes.ErrRepoNotFound
		}
		return nil, gitlabErr(err, resp, false)
	}
	return cut(raw, limit), nil
}

func (g *giteaClient) ListTree(ctx context.Context, repo forgetypes.RemoteRepo, commit string) ([]forgetypes.TreeEntry, bool, error) {
	g.c.SetContext(ctx)
	owner, name := splitFull(repo.FullPath)
	var out []forgetypes.TreeEntry
	seen := 0
	for page := 1; ; page++ {
		tree, resp, err := g.c.GetTrees(owner, name, gitea.ListTreeOptions{Ref: commit, Recursive: true,
			ListOptions: gitea.ListOptions{Page: page, PageSize: pageSize}})
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return nil, false, forgetypes.ErrRepoNotFound
			}
			return nil, false, giteaErr(err, resp, false)
		}
		for _, e := range tree.Entries {
			if e.Type == "blob" && e.Mode != "120000" {
				out = append(out, forgetypes.TreeEntry{Path: e.Path, BlobSHA: e.SHA, Size: e.Size})
			}
		}
		seen += len(tree.Entries)
		if len(tree.Entries) == 0 || seen >= tree.TotalCount {
			return out, tree.Truncated, nil
		}
	}
}

func (g *giteaClient) ReadFile(ctx context.Context, repo forgetypes.RemoteRepo, e forgetypes.TreeEntry, limit int64) ([]byte, error) {
	g.c.SetContext(ctx)
	owner, name := splitFull(repo.FullPath)
	blob, resp, err := g.c.GetBlob(owner, name, e.BlobSHA)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, forgetypes.ErrRepoNotFound
		}
		return nil, giteaErr(err, resp, false)
	}
	raw, err := io.ReadAll(io.LimitReader(base64.NewDecoder(base64.StdEncoding, bytes.NewReader([]byte(blob.Content))), limit+1))
	if err != nil {
		return nil, &forgetypes.Upstream{Detail: "blob is not base64"}
	}
	return raw, nil
}

func (g *githubClient) Repo(ctx context.Context, fullPath string) (forgetypes.RemoteRepo, error) {
	owner, name := splitFull(fullPath)
	r, _, err := g.c.Repositories.Get(ctx, owner, name)
	if err != nil {
		if isStatus(err, http.StatusNotFound) {
			return forgetypes.RemoteRepo{}, forgetypes.ErrRepoNotFound
		}
		return forgetypes.RemoteRepo{}, githubErr(err, false)
	}
	return githubRepo(r), nil
}

func (g *gitlabClient) Repo(ctx context.Context, fullPath string) (forgetypes.RemoteRepo, error) {
	p, resp, err := g.c.Projects.GetProject(fullPath, &gitlab.GetProjectOptions{}, gitlab.WithContext(ctx))
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return forgetypes.RemoteRepo{}, forgetypes.ErrRepoNotFound
		}
		return forgetypes.RemoteRepo{}, gitlabErr(err, resp, false)
	}
	return gitlabRepo(p), nil
}

func (g *giteaClient) Repo(ctx context.Context, fullPath string) (forgetypes.RemoteRepo, error) {
	g.c.SetContext(ctx)
	owner, name := splitFull(fullPath)
	r, resp, err := g.c.GetRepo(owner, name)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return forgetypes.RemoteRepo{}, forgetypes.ErrRepoNotFound
		}
		return forgetypes.RemoteRepo{}, giteaErr(err, resp, false)
	}
	return giteaRepo(r), nil
}
