package links

import "github.com/google/uuid"

type PathNode struct {
	Slug   string
	Labels map[string]string
}

type ProjectBatch struct {
	Name          string
	DefaultBranch *string
	Path          []PathNode
	ProjectData
}

type Badge struct {
	LinkKey string
	KindKey string
	Title   *string
	Icon    LinkIcon
	URL     *string
}

func (b ProjectBatch) Context(id uuid.UUID) Context {
	c := Context{ProjectID: id, ProjectName: b.Name, Labels: map[string]string{}, Vars: map[string]string{},
		RepoFullPath: b.RepoFullPath, Branch: b.DefaultBranch, Deployments: b.Deployments, Kinds: b.Kinds}
	for _, n := range b.Path {
		c.Path = append(c.Path, n.Slug)
		c.ProjectSlug = n.Slug
		for k, v := range n.Labels {
			c.Labels[k] = v
		}
	}
	for _, v := range b.Vars {
		c.Vars[v.Key] = v.Value
	}
	return c
}

func Badges(c Context, templates []Template) []Badge {
	out := []Badge{}
	index := map[string]int{}
	fromEvent := map[string]bool{}
	for _, l := range ExpandProject(c, templates, "") {
		i, seen := index[l.LinkKey]
		if !seen {
			index[l.LinkKey] = len(out)
			fromEvent[l.LinkKey] = l.Source == SourceEvent
			out = append(out, Badge{LinkKey: l.LinkKey, KindKey: l.KindKey, Title: l.Title, Icon: l.Icon, URL: l.URL})
			continue
		}
		if l.URL != nil && (out[i].URL == nil || (l.Source == SourceEvent && !fromEvent[l.LinkKey])) {
			out[i].URL = l.URL
			fromEvent[l.LinkKey] = l.Source == SourceEvent
		}
	}
	return out
}
