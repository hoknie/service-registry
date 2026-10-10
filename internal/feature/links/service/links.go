package service

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	catalogservice "svc-registry/internal/feature/catalog/service"
	"svc-registry/internal/feature/links"
	"svc-registry/internal/platform/apperr"
)

const freshCheckSecs = 30

func (s *Service) ListLinkTemplates(ctx context.Context, p access.Principal, id uuid.UUID) ([]links.Template, error) {
	if _, err := s.catalog.Authorize(ctx, p, catalog.PermRead, id); err != nil {
		return nil, err
	}
	items, err := s.templates.Effective(ctx, id)
	return items, apperr.Wrap(err)
}

func (s *Service) PutLinkTemplate(ctx context.Context, p access.Principal, id uuid.UUID, linkKey string, in links.PutTemplate) (links.Template, error) {
	if _, err := s.catalog.Authorize(ctx, p, catalog.PermWrite, id); err != nil {
		return links.Template{}, err
	}
	t, err := links.ValidateTemplate(id, linkKey, in)
	if err != nil {
		return links.Template{}, apperr.Wrap(err)
	}
	if t.IconFile != nil && (s.icons == nil || !s.icons.Exists(*t.IconFile)) {
		return links.Template{}, apperr.Wrap(links.InvalidTemplateIcon)
	}
	saved, err := s.templates.Put(ctx, t)
	return saved, apperr.Wrap(err)
}

func (s *Service) DeleteLinkTemplate(ctx context.Context, p access.Principal, id uuid.UUID, rawKey string) error {
	if _, err := s.catalog.Authorize(ctx, p, catalog.PermWrite, id); err != nil {
		return err
	}
	key, err := links.ValidateKey(rawKey)
	if err != nil {
		return apperr.New(apperr.NotFound)
	}
	return apperr.Wrap(s.templates.Delete(ctx, id, key))
}

func (s *Service) ListNodeVars(ctx context.Context, p access.Principal, id uuid.UUID) ([]links.Var, error) {
	if _, err := s.catalog.Authorize(ctx, p, catalog.PermRead, id); err != nil {
		return nil, err
	}
	items, err := s.templates.EffectiveVars(ctx, id)
	return items, apperr.Wrap(err)
}

func (s *Service) ReplaceNodeVars(ctx context.Context, p access.Principal, id uuid.UUID, raw map[string]string) ([]links.Var, error) {
	if _, err := s.catalog.Authorize(ctx, p, catalog.PermWrite, id); err != nil {
		return nil, err
	}
	vars, err := links.ValidateVars(raw)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if err := s.templates.ReplaceVars(ctx, id, vars); err != nil {
		return nil, apperr.Wrap(err)
	}
	items, err := s.templates.EffectiveVars(ctx, id)
	return items, apperr.Wrap(err)
}

func (s *Service) PreviewLinkTemplate(ctx context.Context, p access.Principal, id uuid.UUID, in links.Preview) ([]links.Link, error) {
	sc, err := s.catalog.Authorize(ctx, p, catalog.PermRead, id)
	if err != nil {
		return nil, err
	}
	if err := links.CheckTemplate(in.Template); err != nil {
		return nil, apperr.Wrap(err)
	}
	project := sc
	switch {
	case in.ProjectID != nil && *in.ProjectID != id:
		under, err := s.templates.IsUnder(ctx, *in.ProjectID, id)
		if err != nil {
			return nil, apperr.Wrap(err)
		}
		if !under {
			return nil, apperr.New(apperr.NotFound)
		}
		if project, err = s.catalog.Authorize(ctx, p, catalog.PermRead, *in.ProjectID); err != nil {
			return nil, apperr.New(apperr.NotFound)
		}
		if project.Node.Kind != catalog.KindProject {
			return nil, apperr.New(apperr.NotFound)
		}
	case sc.Node.Kind != catalog.KindProject:
		return nil, apperr.Wrap(links.InvalidProject)
	}
	q := links.LinkQuery{Environment: deref(in.Environment)}
	if in.Branch != nil {
		q.Branch = *in.Branch
	}
	c, _, err := s.projectContext(ctx, project, q.Branch)
	if err != nil {
		return nil, err
	}
	template := in.Template
	preview := []links.Template{{NodeID: id, LinkKey: "preview", Template: &template}}
	out := links.Expand(c, preview, q.Environment)
	return out, s.attachChecks(ctx, out)
}

func (s *Service) ProjectLinks(ctx context.Context, p access.Principal, id uuid.UUID, q links.LinkQuery) ([]links.Link, error) {
	out, err := s.expandProject(ctx, p, id, q)
	if err != nil {
		return nil, err
	}
	if err := s.targets.Seen(ctx, links.URLs(out)); err != nil {
		return nil, apperr.Wrap(err)
	}
	return out, s.attachChecks(ctx, out)
}

func (s *Service) CheckLink(ctx context.Context, p access.Principal, id uuid.UUID, linkKey string, q links.LinkQuery) ([]links.Link, error) {
	out, err := s.projectLink(ctx, p, id, linkKey, q)
	if err != nil {
		return nil, err
	}
	urls := links.URLs(out)
	checks := make(map[string]links.Check, len(urls))
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

func (s *Service) checkNow(ctx context.Context, url string) (links.Check, error) {
	fresh, err := s.targets.Fresh(ctx, url, freshCheckSecs)
	if err != nil {
		return links.Check{}, err
	}
	if fresh != nil {
		return *fresh, nil
	}
	cfg := s.cfg.LinkCheck
	return s.targets.Record(ctx, url, s.checker.Check(ctx, url), cfg.IntervalSecs, cfg.History)
}

func (s *Service) LinkChecks(ctx context.Context, p access.Principal, id uuid.UUID, linkKey string, q links.LinkQuery) ([]links.Check, error) {
	out, err := s.projectLink(ctx, p, id, linkKey, q)
	if err != nil {
		return nil, err
	}
	items, err := s.targets.History(ctx, links.URLs(out))
	return items, apperr.Wrap(err)
}

func (s *Service) projectLink(ctx context.Context, p access.Principal, id uuid.UUID, rawKey string, q links.LinkQuery) ([]links.Link, error) {
	all, err := s.expandProject(ctx, p, id, q)
	if err != nil {
		return nil, err
	}
	key, err := links.ValidateKey(rawKey)
	if err != nil {
		return nil, apperr.New(apperr.NotFound)
	}
	var out []links.Link
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

func (s *Service) expandProject(ctx context.Context, p access.Principal, id uuid.UUID, q links.LinkQuery) ([]links.Link, error) {
	sc, err := s.catalog.Authorize(ctx, p, catalog.PermRead, id)
	if err != nil {
		return nil, err
	}
	if sc.Node.Kind != catalog.KindProject {
		return nil, apperr.New(apperr.NotFound)
	}
	c, templates, err := s.projectContext(ctx, sc, q.Branch)
	if err != nil {
		return nil, err
	}
	return links.Expand(c, templates, q.Environment), nil
}

func (s *Service) projectContext(ctx context.Context, sc catalogservice.Scoped, rawBranch string) (links.Context, []links.Template, error) {
	ancestors := make([]catalog.Node, 0, len(sc.Path))
	for _, pn := range sc.Path {
		ancestors = append(ancestors, pn.Node)
	}
	c, templates, err := s.loadContext(ctx, sc.Node, ancestors)
	if err != nil {
		return links.Context{}, nil, err
	}
	if rawBranch != "" {
		name, err := catalog.BranchName(rawBranch)
		if err != nil {
			return links.Context{}, nil, apperr.Wrap(err)
		}
		c.Branch = &name
	}
	return c, templates, nil
}

func (s *Service) loadContext(ctx context.Context, project catalog.Node, ancestors []catalog.Node) (links.Context, []links.Template, error) {
	data, err := s.templates.ProjectData(ctx, project.ID)
	if err != nil {
		return links.Context{}, nil, apperr.Wrap(err)
	}
	c := links.Context{
		ProjectID: project.ID, ProjectSlug: project.Slug, ProjectName: project.Name,
		Labels: map[string]string{}, Vars: map[string]string{},
		RepoFullPath: data.RepoFullPath, Branch: project.Repo.DefaultBranch, Deployments: data.Deployments,
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

func (s *Service) attachChecks(ctx context.Context, out []links.Link) error {
	latest, err := s.targets.Latest(ctx, links.URLs(out))
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

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
