package services

import (
	"context"
	"sync"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/apperr"
)

const freshCheckSecs = 30

func (s *ProjectsService) ProjectLinks(ctx context.Context, p accesstypes.Principal, id uuid.UUID, q linkstypes.LinkQuery) ([]linkstypes.Link, error) {
	out, err := s.expandProject(ctx, p, id, q)
	if err != nil {
		return nil, err
	}
	if err := s.targetRepository.Seen(ctx, linkstypes.URLs(out)); err != nil {
		return nil, apperr.Wrap(err)
	}
	return out, s.attachChecks(ctx, out)
}

func (s *ProjectsService) CheckLink(ctx context.Context, p accesstypes.Principal, id uuid.UUID, linkKey string, q linkstypes.LinkQuery) ([]linkstypes.Link, error) {
	out, err := s.projectLink(ctx, p, id, linkKey, q)
	if err != nil {
		return nil, err
	}
	urls := linkstypes.URLs(out)
	checks := make(map[string]linkstypes.Check, len(urls))
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error
	for _, u := range urls {
		wg.Go(func() {
			c, err := s.checkNow(ctx, u)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				firstErr = err
				return
			}
			checks[u] = c
		})
	}
	wg.Wait()
	if firstErr != nil {
		return nil, apperr.Wrap(firstErr)
	}
	for i := range out {
		if out[i].URL != nil {
			c := checks[*out[i].URL]
			out[i].Check = &c
		}
	}
	return out, nil
}

func (s *ProjectsService) checkNow(ctx context.Context, url string) (linkstypes.Check, error) {
	fresh, err := s.targetRepository.Fresh(ctx, url, freshCheckSecs)
	if err != nil {
		return linkstypes.Check{}, err
	}
	if fresh != nil {
		return *fresh, nil
	}
	cfg := s.cfg.LinkCheck
	return s.targetRepository.Record(ctx, url, s.checker.Check(ctx, url), cfg.IntervalSecs, cfg.History)
}

func (s *ProjectsService) LinkChecks(ctx context.Context, p accesstypes.Principal, id uuid.UUID, linkKey string, q linkstypes.LinkQuery) ([]linkstypes.Check, error) {
	out, err := s.projectLink(ctx, p, id, linkKey, q)
	if err != nil {
		return nil, err
	}
	items, err := s.targetRepository.History(ctx, linkstypes.URLs(out))
	return items, apperr.Wrap(err)
}

func (s *ProjectsService) projectLink(ctx context.Context, p accesstypes.Principal, id uuid.UUID, rawKey string, q linkstypes.LinkQuery) ([]linkstypes.Link, error) {
	all, err := s.expandProject(ctx, p, id, q)
	if err != nil {
		return nil, err
	}
	key, err := linkstypes.ValidateKey(rawKey)
	if err != nil {
		return nil, apperr.New(apperr.NotFound)
	}
	var out []linkstypes.Link
	for _, l := range all {
		if l.LinkKey == key {
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		return nil, apperr.New(apperr.NotFound)
	}
	return out, nil
}

func (s *ProjectsService) expandProject(ctx context.Context, p accesstypes.Principal, id uuid.UUID, q linkstypes.LinkQuery) ([]linkstypes.Link, error) {
	sc, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, id)
	if err != nil {
		return nil, err
	}
	if sc.Node.Kind != catalogtypes.KindProject {
		return nil, apperr.New(apperr.NotFound)
	}
	c, templates, err := s.projectContext(ctx, sc, q.Branch)
	if err != nil {
		return nil, err
	}
	return linkstypes.ExpandProject(c, templates, q.Environment), nil
}

func (s *ProjectsService) projectContext(ctx context.Context, sc catalogservices.Scoped, rawBranch string) (linkstypes.Context, []linkstypes.Template, error) {
	ancestors := make([]catalogtypes.Node, 0, len(sc.Path))
	for _, pn := range sc.Path {
		ancestors = append(ancestors, pn.Node)
	}
	c, templates, err := s.loadContext(ctx, sc.Node, ancestors)
	if err != nil {
		return linkstypes.Context{}, nil, err
	}
	if rawBranch != "" {
		name, err := catalogtypes.BranchName(rawBranch)
		if err != nil {
			return linkstypes.Context{}, nil, apperr.Wrap(err)
		}
		c.Branch = &name
	}
	return c, templates, nil
}

func (s *ProjectsService) loadContext(ctx context.Context, project catalogtypes.Node, ancestors []catalogtypes.Node) (linkstypes.Context, []linkstypes.Template, error) {
	data, err := s.templateRepository.ProjectData(ctx, project.ID)
	if err != nil {
		return linkstypes.Context{}, nil, apperr.Wrap(err)
	}
	c := linkstypes.Context{
		ProjectID: project.ID, ProjectSlug: project.Slug, ProjectName: project.Name,
		Labels: map[string]string{}, Vars: map[string]string{},
		RepoFullPath: data.RepoFullPath, Branch: project.Repo.DefaultBranch, Deployments: data.Deployments,
		Kinds: data.Kinds,
	}
	for _, n := range append(ancestors, project) {
		c.Path = append(c.Path, n.Slug)
		for k, v := range n.Labels {
			c.Labels[k] = v
		}
	}
	for _, v := range data.Vars {
		c.Vars[v.Key] = v.Value
	}
	return c, data.Templates, nil
}

func (s *ProjectsService) attachChecks(ctx context.Context, out []linkstypes.Link) error {
	latest, err := s.targetRepository.Latest(ctx, linkstypes.URLs(out))
	if err != nil {
		return apperr.Wrap(err)
	}
	for i := range out {
		if out[i].URL == nil {
			continue
		}
		if c, ok := latest[*out[i].URL]; ok {
			out[i].Check = &c
		}
	}
	return nil
}
