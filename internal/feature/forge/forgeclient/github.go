package forgeclient

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/go-github/v92/github"

	domain "svc-registry/internal/feature/forge"
)

type githubClient struct {
	c     *github.Client
	owner string
	isOrg *bool
}

func newGithub(hc *http.Client, e domain.Endpoint) (*githubClient, error) {
	base := e.APIURL + "/"
	opts := []github.ClientOptionsFunc{github.WithHTTPClient(hc), github.WithURLs(&base, &base)}
	if e.Token != "" {
		opts = append(opts, github.WithAuthToken(e.Token))
	}
	c, err := github.NewClient(opts...)
	if err != nil {
		return nil, &domain.InternalError{Detail: "github client: " + err.Error()}
	}
	return &githubClient{c: c, owner: e.Owner}, nil
}

func githubErr(err error, ownerCall bool) error {
	var limit *github.RateLimitError
	var abuse *github.AbuseRateLimitError
	var resp *github.ErrorResponse
	switch {
	case err == nil:
		return nil
	case errors.As(err, &limit):
		return &domain.RateLimited{Reset: limit.Rate.Reset.Time}
	case errors.As(err, &abuse):
		r := &domain.RateLimited{}
		if abuse.RetryAfter != nil {
			r.Reset = timeNow().Add(*abuse.RetryAfter)
		}
		return r
	case errors.As(err, &resp):
		return fromStatus(resp.Response, ownerCall, err)
	}
	return network(err)
}

func (g *githubClient) resolve(ctx context.Context) error {
	if g.isOrg != nil {
		return nil
	}
	_, _, err := g.c.Organizations.Get(ctx, g.owner)
	if err == nil {
		yes := true
		g.isOrg = &yes
		return nil
	}
	if mapped := githubErr(err, true); !errors.Is(mapped, domain.ErrOwnerNotFound) {
		return mapped
	}
	if _, _, err := g.c.Users.Get(ctx, g.owner); err != nil {
		return githubErr(err, true)
	}
	no := false
	g.isOrg = &no
	return nil
}

func (g *githubClient) Check(ctx context.Context) error { return g.resolve(ctx) }

func (g *githubClient) ListRepos(ctx context.Context) ([]domain.RemoteRepo, error) {
	if err := g.resolve(ctx); err != nil {
		return nil, err
	}
	var out []domain.RemoteRepo
	page := 1
	for page != 0 {
		var repos []*github.Repository
		var resp *github.Response
		var err error
		list := github.ListOptions{Page: page, PerPage: pageSize}
		if *g.isOrg {
			repos, resp, err = g.c.Repositories.ListByOrg(ctx, g.owner, &github.RepositoryListByOrgOptions{Type: "all", ListOptions: list})
		} else {
			repos, resp, err = g.c.Repositories.ListByUser(ctx, g.owner, &github.RepositoryListByUserOptions{Type: "owner", ListOptions: list})
		}
		if err != nil {
			return nil, githubErr(err, true)
		}
		for _, r := range repos {
			out = append(out, githubRepo(r))
		}
		page = resp.NextPage
	}
	return out, nil
}

func githubRepo(r *github.Repository) domain.RemoteRepo {
	vis := domain.VisibilityPublic
	switch {
	case r.GetVisibility() == "internal":
		vis = domain.VisibilityInternal
	case r.GetVisibility() == "private" || r.GetPrivate():
		vis = domain.VisibilityPrivate
	}
	license := ""
	if l := r.GetLicense(); l != nil {
		license = l.GetSPDXID()
		if license == "" || license == "NOASSERTION" {
			license = l.GetName()
		}
	}
	return domain.RemoteRepo{
		ExternalID:    strconv.FormatInt(r.GetID(), 10),
		FullPath:      r.GetFullName(),
		Name:          r.GetName(),
		Description:   r.GetDescription(),
		Topics:        r.Topics,
		WebURL:        r.GetHTMLURL(),
		DefaultBranch: r.GetDefaultBranch(),
		Archived:      r.GetArchived(),
		Fork:          r.GetFork(),
		Visibility:    vis,
		Stars:         r.GetStargazersCount(),
		License:       license,
		PushedAt:      timePtr(r.GetPushedAt().Time),
		UpdatedAt:     timePtr(r.GetUpdatedAt().Time),
	}
}

func (g *githubClient) Details(ctx context.Context, repo domain.RemoteRepo) (domain.Details, error) {
	owner, name := splitFull(repo.FullPath)
	var d domain.Details
	readme, _, err := g.c.Repositories.GetReadme(ctx, owner, name, nil)
	switch {
	case err == nil:
		content, err := readme.GetContent()
		if err != nil {
			return domain.Details{}, &domain.Upstream{Detail: "readme: " + err.Error()}
		}
		d.Readme, d.Truncated = limitReadme([]byte(content))
	case isStatus(err, http.StatusNotFound):
	default:
		return domain.Details{}, githubErr(err, false)
	}
	langs, _, err := g.c.Repositories.ListLanguages(ctx, owner, name)
	if err != nil && !isStatus(err, http.StatusNotFound) {
		return domain.Details{}, githubErr(err, false)
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

func isStatus(err error, status int) bool {
	var resp *github.ErrorResponse
	return errors.As(err, &resp) && resp.Response != nil && resp.Response.StatusCode == status
}

func (g *githubClient) RegisterHook(ctx context.Context, url, secret string) (string, error) {
	if err := g.resolve(ctx); err != nil {
		return "", err
	}
	if !*g.isOrg {
		return "", &domain.Upstream{Status: http.StatusUnprocessableEntity, Detail: "GitHub user accounts have no owner-wide webhooks"}
	}
	contentType := "json"
	hook, _, err := g.c.Organizations.CreateHook(ctx, g.owner, &github.Hook{
		Name:   new("web"),
		Active: new(true),
		Events: []string{"push", "repository"},
		Config: &github.HookConfig{URL: &url, ContentType: &contentType, Secret: &secret},
	})
	if err != nil {
		return "", githubErr(err, false)
	}
	return strconv.FormatInt(hook.GetID(), 10), nil
}

func (g *githubClient) DeleteHook(ctx context.Context, id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil
	}
	if _, err := g.c.Organizations.DeleteHook(ctx, g.owner, n); err != nil && !isStatus(err, http.StatusNotFound) {
		return githubErr(err, false)
	}
	return nil
}
