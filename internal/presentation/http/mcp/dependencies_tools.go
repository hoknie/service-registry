package mcp

import (
	"context"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"svc-registry/internal/presentation/http/responses"
)

type dependenciesIn struct {
	Project string `json:"project" jsonschema:"Project id or slug path from the top level, e.g. acme/backend/api"`
	Depth   int    `json:"depth,omitempty" jsonschema:"Depth of the graph around the project, 1..3 (default 1)"`
}

func (t *tools) getDependencies(ctx context.Context, _ *sdk.CallToolRequest, in dependenciesIn) (*sdk.CallToolResult, any, error) {
	s, err := resolveProject(ctx, t.deps, t.p, in.Project)
	if err != nil {
		return failed(err)
	}
	list, err := t.deps.Topology.Dependencies.ProjectDependencies(ctx, t.p, s.Node.ID)
	if err != nil {
		return failed(err)
	}
	depth := in.Depth
	if depth == 0 {
		depth = 1
	}
	graph, err := t.deps.Topology.Dependencies.DependencyGraph(ctx, t.p, s.Node.ID, &depth)
	if err != nil {
		return failed(err)
	}
	return jsonResult(map[string]any{"project": pathOf(s), "dependencies": responses.DependenciesOf(list),
		"graph": responses.DependencyGraphOf(graph)}, t.max)
}
