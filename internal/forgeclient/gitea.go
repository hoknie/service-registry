package forgeclient

import (
	"context"
	"net/http"
	"regexp"
	"strconv"

	"code.gitea.io/sdk/gitea"

	domain "svc-registry/internal/forge"
)

type giteaClient struct {
	c     *gitea.Client
	owner string
	isOrg *bool
}

func newGitea(hc *http.Client, e domain.Endpoint) (*giteaClient, error) {
	c, err := gitea.NewClient(e.APIURL, gitea.SetHTTPClient(hc), gitea.SetToken(e.Token), gitea.SetGiteaVersion(""))
	if err != nil {
		return nil, &domain.InternalError{Detail: "gitea client: " + err.Error()}
	}
	return &giteaClient{c: c, owner: e.Owner}, nil
}

func giteaErr(err error, resp *gitea.Response, ownerCall bool) error {
	if err == nil {
		return nil
	}
	if resp != nil && resp.Response != nil {
		return fromStatus(resp.Response, ownerCall, err)
	}
	return network(err)
}

func (g *giteaClient) resolve(ctx context.Context) error {
	g.c.SetContext(ctx)
	if g.isOrg != nil {
		return nil
	}
	_, resp, err := g.c.GetOrg(g.owner)
	if err == nil {
		yes := true
		g.isOrg = &yes
		return nil
	}
	if resp == nil || resp.StatusCode != http.StatusNotFound {
		return giteaErr(err, resp, true)
	}
	if _, resp, err := g.c.GetUserInfo(g.owner); err != nil {
		return giteaErr(err, resp, true)
	}
	no := false
	g.isOrg = &no
	return nil
}

func (g *giteaClient) Check(ctx context.Context) error { return g.resolve(ctx) }

func (g *giteaClient) ListRepos(ctx context.Context) ([]domain.RemoteRepo, error) {
	if err := g.resolve(ctx); err != nil {
		return nil, err
	}
	var out []domain.RemoteRepo
	for page := 1; ; page++ {
		list := gitea.ListOptions{Page: page, PageSize: pageSize}
		var repos []*gitea.Repository
		var resp *gitea.Response
		var err error
		if *g.isOrg {
			repos, resp, err = g.c.ListOrgRepos(g.owner, gitea.ListOrgReposOptions{ListOptions: list})
		} else {
			repos, resp, err = g.c.ListUserRepos(g.owner, gitea.ListReposOptions{ListOptions: list})
		}
		if err != nil {
			return nil, giteaErr(err, resp, true)
		}
		for _, r := range repos {
			out = append(out, giteaRepo(r))
		}
		if len(repos) < pageSize || (resp != nil && resp.NextPage == 0 && resp.LastPage != 0) {
			return out, nil
		}
	}
}

func giteaRepo(r *gitea.Repository) domain.RemoteRepo {
	vis := domain.VisibilityPublic
	switch {
	case r.Internal:
		vis = domain.VisibilityInternal
	case r.Private:
		vis = domain.VisibilityPrivate
	}
	out := domain.RemoteRepo{
		ExternalID:    strconv.FormatInt(r.ID, 10),
		FullPath:      r.FullName,
		Name:          r.Name,
		Description:   r.Description,
		Topics:        r.Topics,
		WebURL:        r.HTMLURL,
		DefaultBranch: r.DefaultBranch,
		Archived:      r.Archived,
		Fork:          r.Fork,
		Visibility:    vis,
		Stars:         r.Stars,
		UpdatedAt:     timePtr(r.Updated),
	}
	if len(r.Licenses) > 0 {
		out.License = r.Licenses[0]
	}
	return out
}

var readmeName = regexp.MustCompile(`(?i)^readme(\.(md|markdown|txt|rst))?$`)

func (g *giteaClient) Details(ctx context.Context, repo domain.RemoteRepo) (domain.Details, error) {
	g.c.SetContext(ctx)
	owner, name := splitFull(repo.FullPath)
	var d domain.Details
	entries, resp, err := g.c.ListContents(owner, name, repo.DefaultBranch, "")
	switch {
	case err == nil:
		for _, e := range entries {
			if e.Type == "file" && readmeName.MatchString(e.Name) {
				raw, resp, err := g.c.GetFile(owner, name, repo.DefaultBranch, e.Path)
				if err != nil {
					return domain.Details{}, giteaErr(err, resp, false)
				}
				d.Readme, d.Truncated = limitReadme(raw)
				break
			}
		}
	case resp != nil && resp.StatusCode == http.StatusNotFound:
	default:
		return domain.Details{}, giteaErr(err, resp, false)
	}
	langs, resp, err := g.c.GetRepoLanguages(owner, name)
	if err != nil && (resp == nil || resp.StatusCode != http.StatusNotFound) {
		return domain.Details{}, giteaErr(err, resp, false)
	}
	sizes := map[string]float64{}
	for k, v := range langs {
		sizes[k] = float64(v)
	}
	d.Languages = domain.Percentages(sizes)
	if repo.License != "" {
		l := repo.License
		d.License = &l
	}
	return d, nil
}

func (g *giteaClient) RegisterHook(ctx context.Context, url, secret string) (string, error) {
	if err := g.resolve(ctx); err != nil {
		return "", err
	}
	if !*g.isOrg {
		return "", &domain.Upstream{Status: http.StatusUnprocessableEntity, Detail: "user accounts have no owner-wide webhooks"}
	}
	hook, resp, err := g.c.CreateOrgHook(g.owner, gitea.CreateHookOption{
		Type:   gitea.HookTypeGitea,
		Config: map[string]string{"url": url, "content_type": "json", "secret": secret},
		Events: []string{"push", "repository"},
		Active: true,
	})
	if err != nil {
		return "", giteaErr(err, resp, false)
	}
	return strconv.FormatInt(hook.ID, 10), nil
}

func (g *giteaClient) DeleteHook(ctx context.Context, id string) error {
	g.c.SetContext(ctx)
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil
	}
	resp, err := g.c.DeleteOrgHook(g.owner, n)
	if err != nil && (resp == nil || resp.StatusCode != http.StatusNotFound) {
		return giteaErr(err, resp, false)
	}
	return nil
}
