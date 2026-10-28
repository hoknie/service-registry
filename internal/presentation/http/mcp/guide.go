package mcp

import (
	"context"
	"embed"
	"slices"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"svc-registry/internal/platform/apperr"
)

//go:embed guide/*.md
var guideFiles embed.FS

const guidePrefix = "svcr://guide/"

type guideIn struct {
	Topic string `json:"topic,omitempty" jsonschema:"overview (default), skills, gateway or docs"`
}

func (t *tools) topics() []string {
	out := []string{"overview"}
	if t.deps.Skills != nil {
		out = append(out, "skills")
	}
	if t.deps.Gateway != nil {
		out = append(out, "gateway")
	}
	return append(out, "docs")
}

func (t *tools) guide(topic string) (string, bool) {
	if topic == "" {
		topic = "overview"
	}
	if !slices.Contains(t.topics(), topic) {
		return "", false
	}
	raw, err := guideFiles.ReadFile("guide/" + topic + ".md")
	if err != nil {
		return "", false
	}
	row := ""
	if t.deps.Skills != nil {
		row = "| Skills | `list_skills`, `get_skill`, `get_skill_file`, `get_skill_folder` |\n"
	}
	if t.deps.Gateway != nil {
		row += "| External MCP servers | `list_mcp_servers`, then the `<server>__<tool>` tools |\n"
	}
	base := strings.TrimSuffix(t.deps.PublicURL, "/")
	if base == "" {
		base = "https://<registry>"
	}
	return strings.NewReplacer("{{skills_row}}", row, "{{topics}}", "`"+strings.Join(t.topics(), "`, `")+"`", "{{base}}", base).
		Replace(string(raw)), true
}

func (t *tools) getGuide(_ context.Context, _ *sdk.CallToolRequest, in guideIn) (*sdk.CallToolResult, any, error) {
	text, ok := t.guide(in.Topic)
	if !ok {
		return failed(apperr.Invalid("not_found", "unknown topic; topics: "+strings.Join(t.topics(), ", ")))
	}
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: text}}}, nil, nil
}

func (t *tools) guideResources() []*sdk.Resource {
	out := make([]*sdk.Resource, 0, 3)
	for _, topic := range t.topics() {
		out = append(out, &sdk.Resource{URI: guidePrefix + topic, Name: "guide/" + topic, Title: "Guide: " + topic, MIMEType: "text/markdown"})
	}
	return out
}

func (t *tools) readGuide(uri string) (*sdk.ReadResourceResult, bool) {
	topic, ok := strings.CutPrefix(uri, guidePrefix)
	if !ok {
		return nil, false
	}
	text, ok := t.guide(topic)
	if !ok {
		return nil, false
	}
	return &sdk.ReadResourceResult{Contents: []*sdk.ResourceContents{{URI: uri, MIMEType: "text/markdown", Text: text}}}, true
}
