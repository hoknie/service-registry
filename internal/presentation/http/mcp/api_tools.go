package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	apistypes "svc-registry/internal/feature/apis/types"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/presentation/http/responses"
	"svc-registry/pkg/ptr"
)

type listAPIsIn struct {
	Project string `json:"project" jsonschema:"Project id or slug path from the top level, e.g. acme/backend/api"`
	Branch  string `json:"branch,omitempty" jsonschema:"Branch; the documentation's default branch when empty"`
	Commit  string `json:"commit,omitempty" jsonschema:"Commit of a collected snapshot of the branch"`
	All     bool   `json:"all,omitempty" jsonschema:"Also list vendored descriptions (third_party, vendor, node_modules, google/… protos)"`
}

type getAPIIn struct {
	Project string `json:"project" jsonschema:"Project id or slug path from the top level, e.g. acme/backend/api"`
	Path    string `json:"path" jsonschema:"Path of the API description in the snapshot, e.g. api/openapi.yaml"`
	Branch  string `json:"branch,omitempty" jsonschema:"Branch; the documentation's default branch when empty"`
	Commit  string `json:"commit,omitempty" jsonschema:"Commit of a collected snapshot of the branch"`
	Item    string `json:"item,omitempty" jsonschema:"One operation ('GET /pets/{id}' or operationId), proto service or message (full name)"`
}

func (t *tools) listAPIs(ctx context.Context, _ *sdk.CallToolRequest, in listAPIsIn) (*sdk.CallToolResult, any, error) {
	s, err := resolveProject(ctx, t.deps, t.p, in.Project)
	if err != nil {
		return failed(err)
	}
	l, err := t.deps.Apis.Specs.ListAPIs(ctx, t.p, s.Node.ID, knowledgeservices.KnowledgeRef{Branch: ptr.NonEmpty(in.Branch), Commit: ptr.NonEmpty(in.Commit)}, in.All)
	if err != nil {
		return failed(err)
	}
	return jsonResult(responses.APIListOf(l), t.max)
}

func (t *tools) getAPI(ctx context.Context, _ *sdk.CallToolRequest, in getAPIIn) (*sdk.CallToolResult, any, error) {
	s, err := resolveProject(ctx, t.deps, t.p, in.Project)
	if err != nil {
		return failed(err)
	}
	_, d, err := t.deps.Apis.Specs.GetAPI(ctx, t.p, s.Node.ID, knowledgeservices.KnowledgeRef{Branch: ptr.NonEmpty(in.Branch), Commit: ptr.NonEmpty(in.Commit)}, in.Path)
	if err != nil {
		return failed(err)
	}
	var b strings.Builder
	if !apiMarkdown(&b, d, in.Item) {
		return failed(apperr.New(apperr.NotFound))
	}
	for i, p := range d.Problems {
		if i == 0 {
			b.WriteString("\n## Problems\n\n")
		}
		fmt.Fprintf(&b, "- %s %s: %s\n", p.Code, p.Path, p.Message)
	}
	return text(b.String(), t.max), nil, nil
}

func apiMarkdown(b *strings.Builder, d apistypes.Document, item string) bool {
	switch {
	case d.OpenAPI != nil && item == "":
		openAPIOverview(b, d)
	case d.OpenAPI != nil:
		i := slices.IndexFunc(d.OpenAPI.Operations, func(o apistypes.Operation) bool {
			return strings.EqualFold(o.Key, item) || ptr.Value(o.OperationID) == item
		})
		if i < 0 {
			return false
		}
		operationMarkdown(b, d.OpenAPI, d.OpenAPI.Operations[i])
	case item == "":
		protoOverview(b, d)
	default:
		return protoItem(b, d.Proto, item)
	}
	return true
}

func openAPIOverview(b *strings.Builder, d apistypes.Document) {
	o := d.OpenAPI
	fmt.Fprintf(b, "# %s %s\n\n%s %s, `%s`\n\n", o.Info.Title, o.Info.Version, d.Format, ptr.Value(d.SpecVersion), d.Path)
	if o.Info.Description != nil {
		fmt.Fprintf(b, "%s\n\n", *o.Info.Description)
	}
	for _, s := range o.Servers {
		fmt.Fprintf(b, "- server: %s\n", s.URL)
	}
	b.WriteString("\n| Operation | Summary |\n|---|---|\n")
	for _, op := range o.Operations {
		fmt.Fprintf(b, "| `%s` | %s |\n", op.Key, cell(ptr.Value(op.Summary)))
	}
}

func operationMarkdown(b *strings.Builder, o *apistypes.OpenAPI, op apistypes.Operation) {
	fmt.Fprintf(b, "# %s\n\n", op.Key)
	if op.Summary != nil {
		fmt.Fprintf(b, "%s\n\n", *op.Summary)
	}
	if op.Description != nil {
		fmt.Fprintf(b, "%s\n\n", *op.Description)
	}
	refs := map[string]bool{}
	if len(op.Parameters) > 0 {
		b.WriteString("## Parameters\n\n| Name | In | Required | Schema |\n|---|---|---|---|\n")
		for _, p := range op.Parameters {
			fmt.Fprintf(b, "| %s | %s | %v | `%s` |\n", p.Name, p.In, p.Required, compact(p.Schema, refs))
		}
	}
	if op.RequestBody != nil {
		b.WriteString("\n## Request body\n\n")
		for _, c := range op.RequestBody.Content {
			fmt.Fprintf(b, "- %s: `%s`\n", c.MediaType, compact(c.Schema, refs))
		}
	}
	b.WriteString("\n## Responses\n\n")
	for _, r := range op.Responses {
		fmt.Fprintf(b, "- %s %s\n", r.Status, ptr.Value(r.Description))
		for _, c := range r.Content {
			fmt.Fprintf(b, "  - %s: `%s`\n", c.MediaType, compact(c.Schema, refs))
		}
	}
	schemasMarkdown(b, o, refs)
}

func schemasMarkdown(b *strings.Builder, o *apistypes.OpenAPI, refs map[string]bool) {
	written := map[string]bool{}
	for {
		pending := ""
		for name := range refs {
			if !written[name] && (pending == "" || name < pending) {
				pending = name
			}
		}
		if pending == "" {
			return
		}
		written[pending] = true
		i := slices.IndexFunc(o.Schemas, func(s apistypes.NamedSchema) bool { return s.Name == pending })
		if i < 0 {
			continue
		}
		if len(written) == 1 {
			b.WriteString("\n## Schemas\n")
		}
		fmt.Fprintf(b, "\n### %s\n\n```json\n%s\n```\n", pending, compact(o.Schemas[i].Schema, refs))
	}
}

func compact(v any, refs map[string]bool) string {
	collectRefs(v, refs)
	out, _ := json.Marshal(v)
	return string(out)
}

func collectRefs(v any, refs map[string]bool) {
	switch x := v.(type) {
	case apistypes.Schema:
		collectRefs(map[string]any(x), refs)
	case map[string]any:
		if r, ok := x["ref"].(string); ok && x["unresolved"] == nil {
			refs[r] = true
		}
		for _, c := range x {
			collectRefs(c, refs)
		}
	case []apistypes.Schema:
		for _, c := range x {
			collectRefs(c, refs)
		}
	}
}

func cell(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "|", "\\|"), "\n", " ")
}

func protoOverview(b *strings.Builder, d apistypes.Document) {
	p := d.Proto
	fmt.Fprintf(b, "# %s\n\n%s, `%s`\n\n", ptr.Value(p.Package), ptr.Value(d.SpecVersion), d.Path)
	for _, s := range p.Services {
		fmt.Fprintf(b, "## service %s\n\n", s.FullName)
		for _, m := range s.Methods {
			fmt.Fprintf(b, "- %s\n", methodLine(m))
		}
		b.WriteString("\n")
	}
	b.WriteString("## Messages\n\n")
	for _, m := range p.Messages {
		fmt.Fprintf(b, "- %s (%d fields)\n", m.FullName, len(m.Fields))
	}
	for _, e := range p.Enums {
		fmt.Fprintf(b, "- enum %s\n", e.FullName)
	}
}

func methodLine(m apistypes.Method) string {
	in, out := m.Input, m.Output
	if m.ClientStreaming {
		in = "stream " + in
	}
	if m.ServerStreaming {
		out = "stream " + out
	}
	line := fmt.Sprintf("rpc %s(%s) returns (%s)", m.Name, in, out)
	if m.HTTP != nil {
		line += fmt.Sprintf(" — %s %s", m.HTTP.Method, m.HTTP.Path)
	}
	return line
}

func protoItem(b *strings.Builder, p *apistypes.Proto, item string) bool {
	name := strings.TrimPrefix(item, ".")
	for _, s := range p.Services {
		if s.FullName == name || s.Name == name {
			fmt.Fprintf(b, "# service %s\n\n%s\n\n", s.FullName, ptr.Value(s.Description))
			for _, m := range s.Methods {
				fmt.Fprintf(b, "- %s\n", methodLine(m))
				if m.Description != nil {
					fmt.Fprintf(b, "  %s\n", *m.Description)
				}
			}
			return true
		}
	}
	for _, m := range p.Messages {
		if m.FullName == name {
			fmt.Fprintf(b, "# message %s\n\n%s\n\n| # | Name | Type | Label | Description |\n|---|---|---|---|---|\n", m.FullName, ptr.Value(m.Description))
			for _, f := range m.Fields {
				fmt.Fprintf(b, "| %d | %s | %s | %s | %s |\n", f.Number, f.Name, f.Type, ptr.Value(f.Label), cell(ptr.Value(f.Description)))
			}
			return true
		}
	}
	return false
}
