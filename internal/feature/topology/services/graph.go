package services

import (
	"cmp"
	"context"
	"slices"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	topologytypes "svc-registry/internal/feature/topology/types"
	"svc-registry/internal/platform/apperr"
)

type graphBuild struct {
	levels map[string]int
	nodes  map[string]topologytypes.GraphNode
	edges  map[[2]string]map[topologytypes.Source]bool
	hidden map[[2]string]bool
}

func newGraphBuild() *graphBuild {
	return &graphBuild{levels: map[string]int{}, nodes: map[string]topologytypes.GraphNode{},
		edges: map[[2]string]map[topologytypes.Source]bool{}, hidden: map[[2]string]bool{}}
}

func (g *graphBuild) node(n topologytypes.GraphNode, level int) {
	if _, ok := g.nodes[n.ID]; !ok {
		g.nodes[n.ID], g.levels[n.ID] = n, level
	}
}

func (s *DependenciesService) DependencyGraph(ctx context.Context, p accesstypes.Principal, id uuid.UUID, depth *int) (topologytypes.Graph, error) {
	d := 1
	if depth != nil {
		d = *depth
	}
	if d < 1 || d > topologytypes.MaxDepth {
		return topologytypes.Graph{}, topologytypes.InvalidDependencyDepth
	}
	sc, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, id)
	if err != nil {
		return topologytypes.Graph{}, err
	}
	g := newGraphBuild()
	if sc.Node.Kind == catalogtypes.KindProject {
		err = s.neighborhood(ctx, p, g, id, d)
	} else {
		err = s.subtree(ctx, p, g, id)
	}
	if err != nil {
		return topologytypes.Graph{}, apperr.Wrap(err)
	}
	return g.result(), nil
}

func (s *DependenciesService) neighborhood(ctx context.Context, p accesstypes.Principal, g *graphBuild, id uuid.UUID, depth int) error {
	start, err := s.describe(ctx, p, []uuid.UUID{id})
	if err != nil {
		return err
	}
	g.node(projectNode(start[id], true), 0)
	frontier := []uuid.UUID{id}
	for level := 1; level <= depth && len(frontier) > 0; level++ {
		edges, err := s.dependencyRepository.Touching(ctx, frontier, false)
		if err != nil {
			return err
		}
		next, err := s.addEdges(ctx, p, g, edges, level, map[uuid.UUID]bool{id: true})
		if err != nil {
			return err
		}
		frontier = next
	}
	return nil
}

func (s *DependenciesService) subtree(ctx context.Context, p accesstypes.Principal, g *graphBuild, root uuid.UUID) error {
	projects, err := s.dependencyRepository.SubtreeProjects(ctx, root)
	if err != nil {
		return err
	}
	known, err := s.describe(ctx, p, projects)
	if err != nil {
		return err
	}
	scope := map[uuid.UUID]bool{}
	var readable []uuid.UUID
	for _, id := range projects {
		if ref, ok := known[id]; ok {
			scope[id] = true
			readable = append(readable, id)
			g.node(projectNode(ref, true), 0)
		}
	}
	if len(readable) == 0 {
		return nil
	}
	edges, err := s.dependencyRepository.Touching(ctx, readable, true)
	if err != nil {
		return err
	}
	_, err = s.addEdges(ctx, p, g, edges, 1, scope)
	return err
}

func (s *DependenciesService) addEdges(ctx context.Context, p accesstypes.Principal, g *graphBuild, edges []topologytypes.Edge, level int, scope map[uuid.UUID]bool) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	for _, e := range edges {
		ids = append(ids, e.ProjectID)
		if e.TargetProjectID != nil {
			ids = append(ids, *e.TargetProjectID)
		}
	}
	known, err := s.describe(ctx, p, ids)
	if err != nil {
		return nil, err
	}
	var next []uuid.UUID
	reach := func(id uuid.UUID) (string, bool) {
		ref, ok := known[id]
		if !ok {
			return "", false
		}
		if _, seen := g.nodes[id.String()]; !seen {
			g.node(projectNode(ref, scope[id]), level)
			next = append(next, id)
		}
		return id.String(), true
	}
	for _, e := range edges {
		from, okFrom := reach(e.ProjectID)
		var to string
		okTo := true
		switch {
		case e.TargetProjectID != nil:
			to, okTo = reach(*e.TargetProjectID)
		case e.TargetExternal != nil:
			to = "external:" + *e.TargetExternal
			g.node(topologytypes.GraphNode{ID: to, Type: topologytypes.TargetExternal, Name: *e.TargetExternal}, level)
		default:
			to = "ref:" + e.TargetRef
			g.node(topologytypes.GraphNode{ID: to, Type: topologytypes.TargetUnresolved, Name: e.TargetRef}, level)
		}
		if !okFrom || !okTo {
			g.hidden[[2]string{e.ProjectID.String(), e.TargetKey()}] = true
			continue
		}
		pair := [2]string{from, to}
		if g.edges[pair] == nil {
			g.edges[pair] = map[topologytypes.Source]bool{}
		}
		g.edges[pair][e.Source] = true
	}
	return next, nil
}

func projectNode(ref topologytypes.ProjectRef, inScope bool) topologytypes.GraphNode {
	return topologytypes.GraphNode{ID: ref.ID.String(), Type: topologytypes.TargetProject, Path: ref.Path, Name: ref.Name, InScope: inScope}
}

func (g *graphBuild) result() topologytypes.Graph {
	out := topologytypes.Graph{Nodes: []topologytypes.GraphNode{}, Edges: []topologytypes.GraphEdge{}, HiddenEdges: len(g.hidden)}
	for _, n := range g.nodes {
		out.Nodes = append(out.Nodes, n)
	}
	slices.SortFunc(out.Nodes, func(a, b topologytypes.GraphNode) int {
		return cmp.Or(cmp.Compare(g.levels[a.ID], g.levels[b.ID]), cmp.Compare(a.Path, b.Path), cmp.Compare(a.Name, b.Name))
	})
	if len(out.Nodes) > topologytypes.MaxGraphNodes {
		out.Nodes, out.Truncated = out.Nodes[:topologytypes.MaxGraphNodes], true
	}
	kept := map[string]bool{}
	for _, n := range out.Nodes {
		kept[n.ID] = true
	}
	for pair, sources := range g.edges {
		if !kept[pair[0]] || !kept[pair[1]] {
			continue
		}
		e := topologytypes.GraphEdge{From: pair[0], To: pair[1]}
		for _, src := range []topologytypes.Source{topologytypes.SourceEvent, topologytypes.SourceCluster, topologytypes.SourceRepository, topologytypes.SourceManual} {
			if sources[src] {
				e.Sources = append(e.Sources, src)
			}
		}
		out.Edges = append(out.Edges, e)
	}
	slices.SortFunc(out.Edges, func(a, b topologytypes.GraphEdge) int {
		return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
	})
	return out
}
