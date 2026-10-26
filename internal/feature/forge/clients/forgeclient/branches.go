package forgeclient

import (
	"context"
	"net/http"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/google/go-github/v92/github"
	gitlab "gitlab.com/gitlab-org/api/client-go"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	forgetypes "svc-registry/internal/feature/forge/types"
)

type collector struct {
	keep      func(string) bool
	max       int
	out       []catalogtypes.ForgeBranch
	truncated bool
}

func (c *collector) add(b catalogtypes.ForgeBranch) bool {
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

func (g *githubClient) ListBranches(ctx context.Context, repo forgetypes.RemoteRepo, keep func(string) bool, limit int) ([]catalogtypes.ForgeBranch, bool, error) {
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
			if !c.add(catalogtypes.ForgeBranch{Name: b.GetName(), HeadSHA: lowerSHA(b.GetCommit().GetSHA()), Protected: b.Protected}) {
				return c.out, true, nil
			}
		}
		if resp.NextPage == 0 {
			return c.out, c.truncated, nil
		}
		opt.Page = resp.NextPage
	}
}

func (g *gitlabClient) ListBranches(ctx context.Context, repo forgetypes.RemoteRepo, keep func(string) bool, limit int) ([]catalogtypes.ForgeBranch, bool, error) {
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
			fb := catalogtypes.ForgeBranch{Name: b.Name, Protected: new(b.Protected)}
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

func (g *giteaClient) ListBranches(ctx context.Context, repo forgetypes.RemoteRepo, keep func(string) bool, limit int) ([]catalogtypes.ForgeBranch, bool, error) {
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
			fb := catalogtypes.ForgeBranch{Name: b.Name, Protected: new(b.Protected)}
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

func tagCollector(keep func(string) bool, limit int) (*collector, func() []forgetypes.Tag) {
	c := &collector{keep: keep, max: limit}
	return c, func() []forgetypes.Tag {
		out := make([]forgetypes.Tag, len(c.out))
		for i, b := range c.out {
			out[i] = forgetypes.Tag{Name: b.Name, Commit: b.HeadSHA}
		}
		return out
	}
}

func (g *githubClient) ListTags(ctx context.Context, repo forgetypes.RemoteRepo, keep func(string) bool, limit int) ([]forgetypes.Tag, bool, error) {
	owner, name := splitFull(repo.FullPath)
	c, result := tagCollector(keep, limit)
	opt := &github.ListOptions{PerPage: 100, Page: 1}
	for {
		tags, resp, err := g.c.Repositories.ListTags(ctx, owner, name, opt)
		if err != nil {
			if isStatus(err, http.StatusNotFound) {
				return nil, false, nil
			}
			return nil, false, githubErr(err, false)
		}
		for _, t := range tags {
			if !c.add(catalogtypes.ForgeBranch{Name: t.GetName(), HeadSHA: lowerSHA(t.GetCommit().GetSHA())}) {
				return result(), true, nil
			}
		}
		if resp.NextPage == 0 {
			return result(), c.truncated, nil
		}
		opt.Page = resp.NextPage
	}
}

func (g *gitlabClient) ListTags(ctx context.Context, repo forgetypes.RemoteRepo, keep func(string) bool, limit int) ([]forgetypes.Tag, bool, error) {
	c, result := tagCollector(keep, limit)
	opt := &gitlab.ListTagsOptions{ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1}}
	for {
		tags, resp, err := g.c.Tags.ListTags(repo.ExternalID, opt, gitlab.WithContext(ctx))
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return nil, false, nil
			}
			return nil, false, gitlabErr(err, resp, false)
		}
		for _, t := range tags {
			b := catalogtypes.ForgeBranch{Name: t.Name}
			if t.Commit != nil {
				b.HeadSHA = lowerSHA(t.Commit.ID)
			}
			if !c.add(b) {
				return result(), true, nil
			}
		}
		if resp.NextPage == 0 {
			return result(), c.truncated, nil
		}
		opt.Page = resp.NextPage
	}
}

func (g *giteaClient) ListTags(ctx context.Context, repo forgetypes.RemoteRepo, keep func(string) bool, limit int) ([]forgetypes.Tag, bool, error) {
	g.c.SetContext(ctx)
	owner, name := splitFull(repo.FullPath)
	c, result := tagCollector(keep, limit)
	for page := 1; ; page++ {
		tags, resp, err := g.c.ListRepoTags(owner, name, gitea.ListRepoTagsOptions{ListOptions: gitea.ListOptions{Page: page, PageSize: pageSize}})
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return nil, false, nil
			}
			return nil, false, giteaErr(err, resp, false)
		}
		for _, t := range tags {
			b := catalogtypes.ForgeBranch{Name: t.Name}
			if t.Commit != nil {
				b.HeadSHA = lowerSHA(t.Commit.SHA)
			}
			if !c.add(b) {
				return result(), true, nil
			}
		}
		if len(tags) < pageSize {
			return result(), c.truncated, nil
		}
	}
}
