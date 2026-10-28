package responses

import (
	"encoding/json"
	"slices"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPBuiltinTool struct {
	Name        string               `json:"name"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	InputSchema any                  `json:"input_schema"`
	Annotations *sdk.ToolAnnotations `json:"annotations"`
	Enabled     bool                 `json:"enabled"`
}

type MCPBuiltin struct {
	ID           string           `json:"id"`
	Slug         string           `json:"slug"`
	Title        string           `json:"title"`
	Builtin      bool             `json:"builtin"`
	Version      string           `json:"version"`
	Instructions string           `json:"instructions"`
	Endpoint     string           `json:"endpoint"`
	Tools        []MCPBuiltinTool `json:"tools"`
	Disabled     []string         `json:"disabled"`
	CanWrite     bool             `json:"can_write"`
}

func MCPBuiltinOf(version, instructions, endpoint string, tools []*sdk.Tool, disabled []string, canWrite bool) MCPBuiltin {
	out := MCPBuiltin{ID: "builtin", Slug: "svc-registry", Title: "Service Registry", Builtin: true, Version: version,
		Instructions: instructions, Endpoint: endpoint, Tools: make([]MCPBuiltinTool, 0, len(tools)), Disabled: disabled, CanWrite: canWrite}
	for _, t := range tools {
		var schema any = t.InputSchema
		if raw, ok := t.InputSchema.(json.RawMessage); ok {
			schema = raw
		}
		out.Tools = append(out.Tools, MCPBuiltinTool{Name: t.Name, Title: t.Title, Description: t.Description, InputSchema: schema,
			Annotations: t.Annotations, Enabled: !slices.Contains(disabled, t.Name)})
	}
	return out
}
