package links

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type Deployment struct {
	Service     string
	Environment string
	Version     string
	Commit      *string
	Cluster     *string
	Namespace   *string
	URL         *string
}

type Context struct {
	ProjectID    uuid.UUID
	ProjectSlug  string
	ProjectName  string
	Path         []string
	Labels       map[string]string
	Vars         map[string]string
	RepoFullPath *string
	Branch       *string
	Deployments  []Deployment
}

type Link struct {
	LinkKey     string
	KindKey     string
	Title       *string
	Icon        LinkIcon
	NodeID      uuid.UUID
	Inherited   bool
	Service     *string
	Environment *string
	URL         *string
	Missing     []string
	Check       *Check
}

func Expand(c Context, templates []Template, environment string) []Link {
	deployments := slices.Clone(c.Deployments)
	if environment != "" {
		deployments = slices.DeleteFunc(deployments, func(d Deployment) bool { return d.Environment != environment })
	}
	slices.SortFunc(deployments, func(a, b Deployment) int {
		return cmp.Or(strings.Compare(a.Service, b.Service), strings.Compare(a.Environment, b.Environment))
	})
	var out []Link
	for _, t := range templates {
		if t.Disabled || t.Template == nil {
			continue
		}
		p, err := Parse(*t.Template)
		if err != nil {
			continue
		}
		base := Link{LinkKey: t.LinkKey, KindKey: t.KindKey, Title: t.Title, Icon: t.LinkIcon(), NodeID: t.NodeID, Inherited: t.Inherited}
		if !usesDeployment(p) || len(deployments) == 0 {
			out = append(out, expandOne(p, base, c, nil))
			continue
		}
		for _, d := range deployments {
			l := base
			l.Service, l.Environment = &d.Service, &d.Environment
			out = append(out, expandOne(p, l, c, &d))
		}
	}
	return out
}

func expandOne(p *Pattern, l Link, c Context, d *Deployment) Link {
	s, missing := p.Expand(c.values(d))
	l.Missing = missing
	if l.Missing == nil {
		l.Missing = []string{}
	}
	if len(missing) == 0 && CheckURL(s) == nil {
		l.URL = &s
	}
	return l
}

func (c Context) values(d *Deployment) Values {
	return func(name string) (string, bool) {
		switch name {
		case VarProjectID:
			return c.ProjectID.String(), true
		case VarProjectSlug:
			return c.ProjectSlug, true
		case VarProjectName:
			return c.ProjectName, true
		case VarPath:
			return strings.Join(c.Path, "/"), true
		case VarRepoFullPath:
			return some(c.RepoFullPath)
		case VarBranch:
			return some(c.Branch)
		}
		if slices.Contains(DeploymentVars, name) {
			if d == nil {
				return "", false
			}
			switch name {
			case VarService:
				return d.Service, true
			case VarEnvironment:
				return d.Environment, true
			case VarVersion:
				return d.Version, true
			case VarCommit:
				return some(d.Commit)
			case VarCluster:
				return some(d.Cluster)
			case VarNamespace:
				return some(d.Namespace)
			case VarURL:
				return some(d.URL)
			}
		}
		if key, ok := strings.CutPrefix(name, "labels."); ok {
			v, ok := c.Labels[key]
			return v, ok
		}
		if key, ok := strings.CutPrefix(name, "vars."); ok {
			v, ok := c.Vars[key]
			return v, ok
		}
		if n, ok := strings.CutPrefix(name, "node."); ok {
			if depth, err := strconv.Atoi(n); err == nil && depth >= 1 && depth <= len(c.Path) {
				return c.Path[depth-1], true
			}
		}
		return "", false
	}
}

func some(v *string) (string, bool) {
	if v == nil || *v == "" {
		return "", false
	}
	return *v, true
}

func URLs(links []Link) []string {
	var out []string
	for _, l := range links {
		if l.URL != nil && !slices.Contains(out, *l.URL) {
			out = append(out, *l.URL)
		}
	}
	return out
}
