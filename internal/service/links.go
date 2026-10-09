package service

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
	"svc-registry/internal/links"
)

const freshCheckSecs = 30

func ListLinkTemplates(ctx context.Context, state *State, p Principal, id uuid.UUID) ([]links.Template, error) {
	if _, err := Authorize(ctx, state, p, catalog.PermRead, id); err != nil {
		return nil, err
	}
	items, err := state.LinkTemplates.Effective(ctx, id)
	return items, apperr.Wrap(err)
}

func PutLinkTemplate(ctx context.Context, state *State, p Principal, id uuid.UUID, linkKey string, in links.PutTemplate) (links.Template, error) {
	if _, err := Authorize(ctx, state, p, catalog.PermWrite, id); err != nil {
		return links.Template{}, err
	}
	t, err := links.ValidateTemplate(id, linkKey, in)
	if err != nil {
		return links.Template{}, apperr.Wrap(err)
	}
	saved, err := state.LinkTemplates.Put(ctx, t)
	return saved, apperr.Wrap(err)
}

func DeleteLinkTemplate(ctx context.Context, state *State, p Principal, id uuid.UUID, rawKey string) error {
	if _, err := Authorize(ctx, state, p, catalog.PermWrite, id); err != nil {
		return err
	}
	key, err := links.ValidateKey(rawKey)
	if err != nil {
		return apperr.New(apperr.NotFound)
	}
	return apperr.Wrap(state.LinkTemplates.Delete(ctx, id, key))
}

func ListNodeVars(ctx context.Context, state *State, p Principal, id uuid.UUID) ([]links.Var, error) {
	if _, err := Authorize(ctx, state, p, catalog.PermRead, id); err != nil {
		return nil, err
	}
	items, err := state.LinkTemplates.EffectiveVars(ctx, id)
	return items, apperr.Wrap(err)
}

func ReplaceNodeVars(ctx context.Context, state *State, p Principal, id uuid.UUID, raw map[string]string) ([]links.Var, error) {
	if _, err := Authorize(ctx, state, p, catalog.PermWrite, id); err != nil {
		return nil, err
	}
	vars, err := links.ValidateVars(raw)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if err := state.LinkTemplates.ReplaceVars(ctx, id, vars); err != nil {
		return nil, apperr.Wrap(err)
	}
	items, err := state.LinkTemplates.EffectiveVars(ctx, id)
	return items, apperr.Wrap(err)
}

func PreviewLinkTemplate(ctx context.Context, state *State, p Principal, id uuid.UUID, in links.Preview) ([]links.Link, error) {
	s, err := Authorize(ctx, state, p, catalog.PermRead, id)
	if err != nil {
		return nil, err
	}
	if err := links.CheckTemplate(in.Template); err != nil {
		return nil, apperr.Wrap(err)
	}
	project := s
	switch {
	case in.ProjectID != nil && *in.ProjectID != id:
		under, err := state.LinkTemplates.IsUnder(ctx, *in.ProjectID, id)
		if err != nil {
			return nil, apperr.Wrap(err)
		}
		if !under {
			return nil, apperr.New(apperr.NotFound)
		}
		if project, err = Authorize(ctx, state, p, catalog.PermRead, *in.ProjectID); err != nil {
			return nil, apperr.New(apperr.NotFound)
		}
		if project.Node.Kind != catalog.KindProject {
			return nil, apperr.New(apperr.NotFound)
		}
	case s.Node.Kind != catalog.KindProject:
		return nil, apperr.Wrap(links.InvalidProject)
	}
	q := links.LinkQuery{Environment: deref(in.Environment)}
	if in.Branch != nil {
		q.Branch = *in.Branch
	}
	c, _, err := projectContext(ctx, state, project, q.Branch)
	if err != nil {
		return nil, err
	}
	template := in.Template
	preview := []links.Template{{NodeID: id, LinkKey: "preview", Template: &template}}
	out := links.Expand(c, preview, q.Environment)
	return out, attachChecks(ctx, state, out)
}

func ProjectLinks(ctx context.Context, state *State, p Principal, id uuid.UUID, q links.LinkQuery) ([]links.Link, error) {
	out, err := expandProject(ctx, state, p, id, q)
	if err != nil {
		return nil, err
	}
	if err := state.LinkTargets.Seen(ctx, links.URLs(out)); err != nil {
		return nil, apperr.Wrap(err)
	}
	return out, attachChecks(ctx, state, out)
}

func CheckLink(ctx context.Context, state *State, p Principal, id uuid.UUID, linkKey string, q links.LinkQuery) ([]links.Link, error) {
	out, err := projectLink(ctx, state, p, id, linkKey, q)
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
			c, err := checkNow(ctx, state, u)
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

func checkNow(ctx context.Context, state *State, url string) (links.Check, error) {
	fresh, err := state.LinkTargets.Fresh(ctx, url, freshCheckSecs)
	if err != nil {
		return links.Check{}, err
	}
	if fresh != nil {
		return *fresh, nil
	}
	cfg := state.Config.LinkCheck
	return state.LinkTargets.Record(ctx, url, state.LinkChecker.Check(ctx, url), cfg.IntervalSecs, cfg.History)
}

func LinkChecks(ctx context.Context, state *State, p Principal, id uuid.UUID, linkKey string, q links.LinkQuery) ([]links.Check, error) {
	out, err := projectLink(ctx, state, p, id, linkKey, q)
	if err != nil {
		return nil, err
	}
	items, err := state.LinkTargets.History(ctx, links.URLs(out))
	return items, apperr.Wrap(err)
}

func projectLink(ctx context.Context, state *State, p Principal, id uuid.UUID, rawKey string, q links.LinkQuery) ([]links.Link, error) {
	all, err := expandProject(ctx, state, p, id, q)
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

func expandProject(ctx context.Context, state *State, p Principal, id uuid.UUID, q links.LinkQuery) ([]links.Link, error) {
	s, err := Authorize(ctx, state, p, catalog.PermRead, id)
	if err != nil {
		return nil, err
	}
	if s.Node.Kind != catalog.KindProject {
		return nil, apperr.New(apperr.NotFound)
	}
	c, templates, err := projectContext(ctx, state, s, q.Branch)
	if err != nil {
		return nil, err
	}
	return links.Expand(c, templates, q.Environment), nil
}

func projectContext(ctx context.Context, state *State, s Scoped, rawBranch string) (links.Context, []links.Template, error) {
	ancestors := make([]catalog.Node, 0, len(s.Path))
	for _, pn := range s.Path {
		ancestors = append(ancestors, pn.Node)
	}
	c, templates, err := loadContext(ctx, state, s.Node, ancestors)
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

func loadContext(ctx context.Context, state *State, project catalog.Node, ancestors []catalog.Node) (links.Context, []links.Template, error) {
	data, err := state.LinkTemplates.ProjectData(ctx, project.ID)
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

func attachChecks(ctx context.Context, state *State, out []links.Link) error {
	latest, err := state.LinkTargets.Latest(ctx, links.URLs(out))
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
