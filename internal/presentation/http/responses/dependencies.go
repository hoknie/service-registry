package responses

import (
	"github.com/google/uuid"

	topologytypes "svc-registry/internal/feature/topology/types"
)

type DependencySource struct {
	Source string     `json:"source"`
	Origin string     `json:"origin"`
	SeenAt string     `json:"seen_at"`
	ID     *uuid.UUID `json:"id,omitempty"`
	Note   *string    `json:"note,omitempty"`
}

type DependencyProject struct {
	ID   uuid.UUID `json:"id"`
	Path string    `json:"path"`
	Name string    `json:"name"`
}

type DependencyTarget struct {
	Type string     `json:"type"`
	ID   *uuid.UUID `json:"id,omitempty"`
	Path *string    `json:"path,omitempty"`
	Name *string    `json:"name,omitempty"`
	Ref  *string    `json:"ref,omitempty"`
}

type DependencyItem struct {
	Target  DependencyTarget   `json:"target"`
	Sources []DependencySource `json:"sources"`
}

type Dependent struct {
	Project DependencyProject  `json:"project"`
	Sources []DependencySource `json:"sources"`
}

type Dependencies struct {
	DependsOn        []DependencyItem `json:"depends_on"`
	Dependents       []Dependent      `json:"dependents"`
	HiddenDependents int              `json:"hidden_dependents"`
}

type DependencyNode struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Path    string `json:"path"`
	Name    string `json:"name"`
	InScope bool   `json:"in_scope"`
}

type DependencyEdge struct {
	From    string   `json:"from"`
	To      string   `json:"to"`
	Sources []string `json:"sources"`
}

type DependencyGraph struct {
	Nodes       []DependencyNode `json:"nodes"`
	Edges       []DependencyEdge `json:"edges"`
	HiddenEdges int              `json:"hidden_edges"`
	Truncated   bool             `json:"truncated"`
}

func dependencySources(in []topologytypes.SourceRef) []DependencySource {
	out := make([]DependencySource, 0, len(in))
	for _, s := range in {
		out = append(out, DependencySource{Source: string(s.Source), Origin: s.Origin, SeenAt: s.SeenAt, ID: s.ID, Note: s.Note})
	}
	return out
}

func DependencyItemOf(it topologytypes.Item) DependencyItem {
	t := DependencyTarget{Type: it.Target.Type}
	switch it.Target.Type {
	case topologytypes.TargetProject:
		p := it.Target.Project
		t.ID, t.Path, t.Name = &p.ID, &p.Path, &p.Name
	case topologytypes.TargetExternal:
		t.Name = &it.Target.External
	case topologytypes.TargetUnresolved:
		t.Ref = &it.Target.Ref
	}
	return DependencyItem{Target: t, Sources: dependencySources(it.Sources)}
}

func DependenciesOf(l topologytypes.List) Dependencies {
	out := Dependencies{DependsOn: ItemsOf(l.DependsOn, DependencyItemOf).Items, Dependents: []Dependent{}, HiddenDependents: l.HiddenDependents}
	for _, d := range l.Dependents {
		out.Dependents = append(out.Dependents, Dependent{Project: DependencyProject(d.Project), Sources: dependencySources(d.Sources)})
	}
	return out
}

func DependencyGraphOf(g topologytypes.Graph) DependencyGraph {
	out := DependencyGraph{Nodes: []DependencyNode{}, Edges: []DependencyEdge{}, HiddenEdges: g.HiddenEdges, Truncated: g.Truncated}
	for _, n := range g.Nodes {
		out.Nodes = append(out.Nodes, DependencyNode(n))
	}
	for _, e := range g.Edges {
		sources := make([]string, 0, len(e.Sources))
		for _, s := range e.Sources {
			sources = append(sources, string(s))
		}
		out.Edges = append(out.Edges, DependencyEdge{From: e.From, To: e.To, Sources: sources})
	}
	return out
}
