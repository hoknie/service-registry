package mcp

import (
	"context"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	catalogservices "svc-registry/internal/feature/catalog/services"
)

type callKey struct{}

type call struct{ organization, project string }

func remember(ctx context.Context, s catalogservices.Scoped) {
	c, ok := ctx.Value(callKey{}).(*call)
	if !ok {
		return
	}
	c.project, c.organization = s.Node.ID.String(), s.Node.ID.String()
	if len(s.Path) > 0 {
		c.organization = s.Path[0].Node.ID.String()
	}
}

func (t *tools) finish(ctx context.Context, kind, name string, failed bool, start time.Time) {
	t.deps.Metrics.MCPCall(kind, name, failed, time.Since(start))
	if c, ok := ctx.Value(callKey{}).(*call); ok && c.project != "" {
		t.deps.Metrics.MCPProjectCall(c.organization, c.project)
	}
}

func observe[In, Out any](t *tools, name string, h sdk.ToolHandlerFor[In, Out]) sdk.ToolHandlerFor[In, Out] {
	if t.deps.Metrics == nil {
		return h
	}
	return func(ctx context.Context, req *sdk.CallToolRequest, in In) (*sdk.CallToolResult, Out, error) {
		ctx = context.WithValue(ctx, callKey{}, &call{})
		start := time.Now()
		res, out, err := h(ctx, req, in)
		t.finish(ctx, "tool", name, err != nil || res != nil && res.IsError, start)
		return res, out, err
	}
}

func (t *tools) observeResource(name string, h sdk.ResourceHandler) sdk.ResourceHandler {
	if t.deps.Metrics == nil {
		return h
	}
	return func(ctx context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
		ctx = context.WithValue(ctx, callKey{}, &call{})
		start := time.Now()
		res, err := h(ctx, req)
		t.finish(ctx, "resource", name, err != nil, start)
		return res, err
	}
}

func searchMode(deps Deps, requested string) string {
	if requested != "" {
		return requested
	}
	return string(deps.Knowledge.Search.DefaultSearchMode())
}
