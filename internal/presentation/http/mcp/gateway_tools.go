package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	gatewayservices "svc-registry/internal/feature/gateway/services"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/apperr"
)

type serversIn struct {
	Query string `json:"query,omitempty" jsonschema:"Substring of the slug, title or description of a server"`
}

type serverOffer struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	State       string   `json:"state"`
	Prefix      string   `json:"prefix"`
	Tools       []string `json:"tools"`
	Note        string   `json:"note,omitempty"`
}

func calledTool(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	body, err := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	var msg struct {
		Method string `json:"method"`
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	if json.Unmarshal(body, &msg) != nil || msg.Method != "tools/call" {
		return ""
	}
	return msg.Params.Name
}

func (t *tools) registerGateway(ctx context.Context, s *sdk.Server, called string) {
	if t.deps.Gateway == nil {
		return
	}
	sdk.AddTool(s, &sdk.Tool{Name: "list_mcp_servers", Description: "External MCP servers available to you through this registry, " +
		"with the prefix of their tools and the tool names. Their tools are listed by tools/list as <slug>__<tool>." + dataNote,
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true}}, observe(t, "list_mcp_servers", t.listServers))
	offered, err := t.deps.Gateway.Gateway.Tools(ctx, t.p)
	if err != nil {
		offered = nil
	}
	known := false
	for _, g := range offered {
		def := &sdk.Tool{Name: g.Name, Title: g.Title, Description: g.Description, InputSchema: g.InputSchema}
		if len(g.OutputSchema) > 0 {
			def.OutputSchema = g.OutputSchema
		}
		if len(g.Annotations) > 0 {
			var a sdk.ToolAnnotations
			if json.Unmarshal(g.Annotations, &a) == nil {
				def.Annotations = &a
			}
		}
		s.AddTool(def, t.gatewayCall(g.Name))
		known = known || g.Name == called
	}
	if _, _, ok := gatewaytypes.SplitGatewayName(called); ok && !known {
		s.AddTool(&sdk.Tool{Name: called, InputSchema: map[string]any{"type": "object"}}, t.gatewayCall(called))
	}
}

func (t *tools) gatewayCall(name string) sdk.ToolHandler {
	return func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		start := time.Now()
		res := t.forward(ctx, name, req.Params.Arguments)
		if t.deps.Metrics != nil {
			t.deps.Metrics.MCPCall("tool", "gateway", res.IsError, time.Since(start))
		}
		return res, nil
	}
}

func (t *tools) forward(ctx context.Context, name string, args json.RawMessage) *sdk.CallToolResult {
	out, err := t.deps.Gateway.Gateway.Call(ctx, t.p, name, args)
	var f *gatewaytypes.Failure
	switch {
	case errors.As(err, &f):
		return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: f.Error()}}}
	case err != nil:
		res, _, _ := failed(err)
		return res
	}
	var res sdk.CallToolResult
	if json.Unmarshal(out.Raw, &res) != nil {
		res, _, _ := failed(apperr.InternalDetail("unreadable gateway result"))
		return res
	}
	if res.Content == nil {
		res.Content = []sdk.Content{}
	}
	return &res
}

func (t *tools) listServers(ctx context.Context, _ *sdk.CallToolRequest, in serversIn) (*sdk.CallToolResult, any, error) {
	found, err := t.deps.Gateway.Gateway.Servers(ctx, t.p, in.Query)
	if err != nil {
		return failed(err)
	}
	out := make([]serverOffer, len(found))
	for i, o := range found {
		out[i] = offerOf(o)
	}
	return jsonResult(map[string]any{"servers": out}, t.max)
}

func offerOf(o gatewayservices.Offer) serverOffer {
	names := make([]string, len(o.Tools))
	for i, tool := range o.Tools {
		names[i] = tool.GatewayName
	}
	out := serverOffer{Slug: o.Server.Slug, Title: o.Server.Title, Description: strings.TrimSpace(o.Server.Description),
		State: string(o.Server.State()), Prefix: o.Server.Slug + "__", Tools: names}
	if o.Server.State() == gatewaytypes.StateFailing {
		out.Note = "the last check of this server failed; its calls may fail now"
	}
	return out
}
