package forgeclient

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	domain "svc-registry/internal/feature/forge"
)

type gitlabClient struct {
	c     *gitlab.Client
	group string
}

func newGitlab(hc *http.Client, e domain.Endpoint) (*gitlabClient, error) {
	c, err := gitlab.NewClient(e.Token, gitlab.WithBaseURL(e.APIURL), gitlab.WithHTTPClient(hc), gitlab.WithoutRetries())
	if err != nil {
		return nil, &domain.InternalError{Detail: "gitlab client: " + err.Error()}
	}
	return &gitlabClient{c: c, group: e.Owner}, nil
}

func gitlabErr(err error, resp *gitlab.Response, ownerCall bool) error {
	if err == nil {
		return nil
	}
	var er *gitlab.ErrorResponse
	if errors.As(err, &er) && er.Response != nil {
		return fromStatus(er.Response, ownerCall, err)
	}
	if resp != nil && resp.Response != nil && resp.StatusCode >= 400 {
		return fromStatus(resp.Response, ownerCall, err)
	}
	return network(err)
}

func (g *gitlabClient) Check(ctx context.Context) error {
	_, resp, err := g.c.Groups.GetGroup(g.group, &gitlab.GetGroupOptions{}, gitlab.WithContext(ctx))
	return gitlabErr(err, resp, true)
}

func (g *gitlabClient) ListRepos(ctx context.Context) ([]domain.RemoteRepo, error) {
	var out []domain.RemoteRepo
	opt := &gitlab.ListGroupProjectsOptions{
		IncludeSubGroups: gitlab.Ptr(true),
		ListOptions:      gitlab.ListOptions{PerPage: pageSize, Page: 1},
	}
	for {
		projects, resp, err := g.c.Groups.ListGroupProjects(g.group, opt, gitlab.WithContext(ctx))
		if err != nil {
			return nil, gitlabErr(err, resp, true)
		}
		for _, p := range projects {
			out = append(out, gitlabRepo(p))
		}
		if resp.NextPage == 0 {
			return out, nil
		}
		opt.Page = resp.NextPage
	}
}

func gitlabRepo(p *gitlab.Project) domain.RemoteRepo {
	vis := domain.VisibilityPrivate
	switch p.Visibility {
	case gitlab.PublicVisibility:
		vis = domain.VisibilityPublic
	case gitlab.InternalVisibility:
		vis = domain.VisibilityInternal
	}
	r := domain.RemoteRepo{
		ExternalID:    strconv.FormatInt(p.ID, 10),
		FullPath:      p.PathWithNamespace,
		Name:          p.Name,
		Description:   p.Description,
		Topics:        p.Topics,
		WebURL:        p.WebURL,
		DefaultBranch: p.DefaultBranch,
		Archived:      p.Archived,
		Fork:          p.ForkedFromProject != nil,
		Visibility:    vis,
		Stars:         int(p.StarCount),
		PushedAt:      p.LastActivityAt,
		UpdatedAt:     p.UpdatedAt,
		ReadmeHint:    p.ReadmeURL,
	}
	if p.License != nil {
		r.License = p.License.Key
	}
	return r
}

func readmePath(hint, branch string) string {
	_, rest, ok := strings.Cut(hint, "/-/blob/")
	if !ok {
		return ""
	}
	if after, ok := strings.CutPrefix(rest, branch+"/"); ok {
		return after
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return rest[i+1:]
	}
	return ""
}

func (g *gitlabClient) Details(ctx context.Context, repo domain.RemoteRepo) (domain.Details, error) {
	var d domain.Details
	id := repo.ExternalID
	if p := readmePath(repo.ReadmeHint, repo.DefaultBranch); p != "" {
		opt := &gitlab.GetRawFileOptions{}
		if repo.DefaultBranch != "" {
			opt.Ref = gitlab.Ptr(repo.DefaultBranch)
		}
		raw, resp, err := g.c.RepositoryFiles.GetRawFile(id, p, opt, gitlab.WithContext(ctx))
		switch {
		case err == nil:
			d.Readme, d.Truncated = limitReadme(raw)
		case resp != nil && resp.StatusCode == http.StatusNotFound:
		default:
			return domain.Details{}, gitlabErr(err, resp, false)
		}
	}
	langs, resp, err := g.c.Projects.GetProjectLanguages(id, gitlab.WithContext(ctx))
	if err != nil {
		return domain.Details{}, gitlabErr(err, resp, false)
	}
	shares := map[string]float64{}
	if langs != nil {
		for k, v := range *langs {
			shares[k] = float64(v)
		}
	}
	d.Languages = domain.Percentages(shares)
	project, resp, err := g.c.Projects.GetProject(id, &gitlab.GetProjectOptions{License: gitlab.Ptr(true)}, gitlab.WithContext(ctx))
	if err != nil {
		return domain.Details{}, gitlabErr(err, resp, false)
	}
	if project.License != nil {
		l := project.License.Key
		if l == "" {
			l = project.License.Name
		}
		if l != "" {
			d.License = &l
		}
	}
	return d, nil
}

func (g *gitlabClient) RegisterHook(ctx context.Context, url, secret string) (string, error) {
	hook, resp, err := g.c.Groups.AddGroupHook(g.group, &gitlab.AddGroupHookOptions{
		URL:                   gitlab.Ptr(url),
		Token:                 gitlab.Ptr(secret),
		PushEvents:            gitlab.Ptr(true),
		ProjectEvents:         gitlab.Ptr(true),
		SubGroupEvents:        gitlab.Ptr(true),
		EnableSSLVerification: gitlab.Ptr(true),
	}, gitlab.WithContext(ctx))
	if err != nil {
		return "", gitlabErr(err, resp, false)
	}
	return strconv.FormatInt(hook.ID, 10), nil
}

func (g *gitlabClient) DeleteHook(ctx context.Context, id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil
	}
	resp, err := g.c.Groups.DeleteGroupHook(g.group, n, gitlab.WithContext(ctx))
	if err != nil && (resp == nil || resp.StatusCode != http.StatusNotFound) {
		return gitlabErr(err, resp, false)
	}
	return nil
}
