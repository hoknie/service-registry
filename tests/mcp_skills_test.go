package tests

import (
	"context"
	"slices"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPSkillsAreFoundAndInstalled(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	a.user("ed@example.com", false)
	a.grant(admin, acme, "user", "ed@example.com", "editor")
	ed := a.session("ed@example.com", password)
	r := a.uploadSkill(ed, map[string]string{"go-testing/SKILL.md": skillMD("go-testing", "  tags: [go]\n  version: 1.3.0\n"),
		"go-testing/reference.md": "# Reference\nUse t.Run.", "go-testing/scripts/run.sh": "#!/bin/sh\n", "go-testing/logo.png": string(tinyPNG)},
		"organization_id", acme, "status", "published")
	eq(t, r.status, 201, r.text())
	id := idOf(r.json(t))
	a.skillVersions(ed, id, "2.0.0")
	a.skill(admin, "go-testing", "status", "published")
	draft := idOf(a.skill(ed, "secret-draft", "organization_id", acme))
	token := a.pat(ed, "mcp")
	s := a.mcpSession(token)
	contains(t, s.InitializeResult().Instructions, "get_guide")
	contains(t, s.InitializeResult().Instructions, "list_skills")

	out, isErr := call(t, s, "list_skills", map[string]any{"tag": []string{"go"}})
	eq(t, isErr, false, out)
	contains(t, out, `"path": "go-testing"`)
	eq(t, strings.Contains(out, "secret-draft"), false)

	out, isErr = call(t, s, "get_skill", map[string]any{"skill": "go-testing"})
	eq(t, isErr, true)
	contains(t, out, "skill_ambiguous")
	contains(t, out, id)

	out, isErr = call(t, s, "get_skill", map[string]any{"skill": id, "version": "^1"})
	eq(t, isErr, false, out)
	contains(t, out, "version: 1.3.0")
	contains(t, out, "available versions: 2.0.0, 1.3.0")
	contains(t, out, "name: go-testing")
	contains(t, out, "Use t.Run.")
	contains(t, out, "#!/bin/sh")
	contains(t, out, "~/.claude/skills/go-testing/")
	contains(t, out, "logo.png")
	eq(t, a.get(skillPath(id), ed).json(t)["downloads"], any(float64(1)))

	out, isErr = call(t, s, "get_skill", map[string]any{"skill": id, "version": "3.0.0"})
	eq(t, isErr, true)
	contains(t, out, "not_found")
	out, isErr = call(t, s, "get_skill", map[string]any{"skill": draft})
	eq(t, isErr, true)
	contains(t, out, "not_found")

	res, err := s.CallTool(context.Background(), &sdk.CallToolParams{Name: "get_skill_file", Arguments: map[string]any{"skill": id, "path": "logo.png", "version": "1.3.0"}})
	must(t, err)
	blob, ok := res.Content[0].(*sdk.EmbeddedResource)
	eq(t, ok, true, jsonText(res.Content))
	eq(t, blob.Resource.MIMEType, "image/png")
	eq(t, string(blob.Resource.Blob), string(tinyPNG))

	read, err := s.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "svcr://skills/" + id + "/scripts/run.sh?version=1.3.0"})
	must(t, err)
	eq(t, read.Contents[0].Text, "#!/bin/sh\n")
	eq(t, a.get(skillPath(id), ed).json(t)["downloads"], any(float64(1)), "files do not count")

	out, isErr = call(t, s, "get_guide", map[string]any{})
	eq(t, isErr, false)
	contains(t, out, "list_skills")
	out, _ = call(t, s, "get_guide", map[string]any{"topic": "skills"})
	contains(t, out, "/api/v1/skills/<id>/archive")
	out, isErr = call(t, s, "get_guide", map[string]any{"topic": "billing"})
	eq(t, isErr, true)
	contains(t, out, "not_found")
	resources, err := s.ListResources(context.Background(), nil)
	must(t, err)
	eq(t, resources.Resources[0].URI, "svcr://guide/overview")
	guide, err := s.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "svcr://guide/docs"})
	must(t, err)
	contains(t, guide.Contents[0].Text, "search_docs")
}

func TestMCPWithoutSkills(t *testing.T) {
	t.Parallel()
	a := startApp(t, "SKILLS_ENABLED", "false", "MCP_GATEWAY_ENABLED", "false")
	admin := a.admin()
	s := a.mcpSession(a.pat(admin, "mcp"))
	tools, err := s.ListTools(context.Background(), nil)
	must(t, err)
	var names []string
	for _, tl := range tools.Tools {
		names = append(names, tl.Name)
	}
	eq(t, len(names), 17)
	eq(t, slices.Contains(names, "list_skills"), false)
	out, isErr := call(t, s, "get_guide", map[string]any{"topic": "skills"})
	eq(t, isErr, true)
	contains(t, out, "not_found")
}
