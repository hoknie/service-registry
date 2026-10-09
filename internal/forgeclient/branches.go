package forgeclient

import (
	"context"
	"net/http"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/google/go-github/v92/github"
	gitlab "gitlab.com/gitlab-org/api/client-go"

	"svc-registry/internal/catalog"
	domain "svc-registry/internal/forge"
)

type collector struct {
	keep      func(string) bool
	max       int
	out       []catalog.ForgeBranch
	truncated bool
}

func (c *collector) add(b catalog.ForgeBranch) bool {
	if !c.keep(b.Name) {
		return true
	}
	if len(c.out) >= c.max {
		c.truncated = true
		return false
	}
	c.out = append(c.out, b)
	return true
}

func lowerSHA(s string) string { return strings.ToLower(s) }

func (g *githubClient) ListBranches(ctx context.Context, repo domain.RemoteRepo, keep func(string) bool, limit int) ([]catalog.ForgeBranch, bool, error) {
	owner, name := splitFull(repo.FullPath)
	c := &collector{keep: keep, max: limit}
	opt := &github.BranchListOptions{ListOptions: github.ListOptions{PerPage: 100, Page: 1}}
	for {
		branches, resp, err := g.c.Repositories.ListBranches(ctx, owner, name, opt)
		if err != nil {
			if isStatus(err, http.StatusNotFound) {
				return nil, false, nil
			}
			return nil, false, githubErr(err, false)
		}
		for _, b := range branches {
			if !c.add(catalog.ForgeBranch{Name: b.GetName(), HeadSHA: lowerSHA(b.GetCommit().GetSHA()), Protected: b.Protected}) {
				return c.out, true, nil
			}
		}
		if resp.NextPage == 0 {
			return c.out, c.truncated, nil
		}
		opt.Page = resp.NextPage
	}
}

func (g *gitlabClient) ListBranches(ctx context.Context, repo domain.RemoteRepo, keep func(string) bool, limit int) ([]catalog.ForgeBranch, bool, error) {
	c := &collector{keep: keep, max: limit}
	opt := &gitlab.ListBranchesOptions{ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1}}
	for {
		branches, resp, err := g.c.Branches.ListBranches(repo.ExternalID, opt, gitlab.WithContext(ctx))
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return nil, false, nil
			}
			return nil, false, gitlabErr(err, resp, false)
		}
		for _, b := range branches {
			protected := b.Protected
			fb := catalog.ForgeBranch{Name: b.Name, Protected: &protected}
			if b.Commit != nil {
				fb.HeadSHA, fb.CommittedAt = lowerSHA(b.Commit.ID), b.Commit.CommittedDate
			}
			if !c.add(fb) {
				return c.out, true, nil
			}
		}
		if resp.NextPage == 0 {
			return c.out, c.truncated, nil
		}
		opt.Page = resp.NextPage
	}
}

func (g *giteaClient) ListBranches(ctx context.Context, repo domain.RemoteRepo, keep func(string) bool, limit int) ([]catalog.ForgeBranch, bool, error) {
	g.c.SetContext(ctx)
	owner, name := splitFull(repo.FullPath)
	c := &collector{keep: keep, max: limit}
	for page := 1; ; page++ {
		branches, resp, err := g.c.ListRepoBranches(owner, name, gitea.ListRepoBranchesOptions{ListOptions: gitea.ListOptions{Page: page, PageSize: pageSize}})
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return nil, false, nil
			}
			return nil, false, giteaErr(err, resp, false)
		}
		for _, b := range branches {
			protected := b.Protected
			fb := catalog.ForgeBranch{Name: b.Name, Protected: &protected}
			if b.Commit != nil {
				fb.HeadSHA, fb.CommittedAt = lowerSHA(b.Commit.ID), timePtr(b.Commit.Timestamp)
			}
			if !c.add(fb) {
				return c.out, true, nil
			}
		}
		if len(branches) < pageSize {
			return c.out, c.truncated, nil
		}
	}
}
