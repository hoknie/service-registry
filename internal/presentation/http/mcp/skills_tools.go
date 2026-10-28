package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	skillsservices "svc-registry/internal/feature/skills/services"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
)

const skillsTemplate = "svcr://skills/{skill}/{+path}"

type (
	listSkillsIn struct {
		Query  string   `json:"query,omitempty" jsonschema:"Words of the name, title, description or tags"`
		Tag    []string `json:"tag,omitempty" jsonschema:"Tags the skill must all have"`
		Folder string   `json:"folder,omitempty" jsonschema:"Only below this skill folder (id or path of slugs)"`
		Limit  int      `json:"limit,omitempty" jsonschema:"At most this many skills, 1..100 (default 50)"`
		Cursor string   `json:"cursor,omitempty" jsonschema:"The next_cursor of a previous call"`
	}
	skillIn struct {
		Skill   string `json:"skill" jsonschema:"Skill id, path (folder/sub/name) or name from SKILL.md"`
		Version string `json:"version,omitempty" jsonschema:"Exact version (1.4.0) or range (^1.2); default: the latest available"`
	}
	skillFileIn struct {
		Skill   string `json:"skill" jsonschema:"Skill id, path (folder/sub/name) or name from SKILL.md"`
		Path    string `json:"path" jsonschema:"Path of the file inside the skill, e.g. scripts/run.sh"`
		Version string `json:"version,omitempty" jsonschema:"Exact version (1.4.0) or range (^1.2); default: the latest available"`
	}
	skillSummary struct {
		ID          uuid.UUID `json:"id"`
		Name        string    `json:"name"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		Tags        []string  `json:"tags"`
		Path        string    `json:"path"`
		Author      string    `json:"author"`
		UpdatedAt   string    `json:"updated_at"`
		Downloads   int64     `json:"downloads"`
		Version     *string   `json:"version"`
		Versions    []string  `json:"versions"`
	}
)

func (t *tools) registerSkills(s *sdk.Server, tool func(name, description string) *sdk.Tool) {
	if t.deps.Skills == nil {
		return
	}
	sdk.AddTool(s, tool("list_skills", "AI skills (Agent Skills folders with SKILL.md) you can install, with their available versions."), observe(t, "list_skills", t.listSkills))
	sdk.AddTool(s, tool("get_skill", "A skill to install: SKILL.md, its files with the text ones included, and how to install it; counts as a download."), observe(t, "get_skill", t.getSkill))
	sdk.AddTool(s, tool("get_skill_file", "One file of a skill; binary files come as base64 resource contents."), observe(t, "get_skill_file", t.getSkillFile))
	sdk.AddTool(s, tool("get_skill_folder", "All installable skills of a skill folder and one command that installs them together."), observe(t, "get_skill_folder", t.getSkillFolder))
	s.AddResourceTemplate(&sdk.ResourceTemplate{Name: "skills", Title: "Skill files", URITemplate: skillsTemplate,
		Description: "A file of a published skill; add ?version=<semver> for another version." + dataNote},
		t.observeResource(skillsTemplate, t.readSkillResource))
}

func available(s skillstypes.Skill) []string {
	out := []string{}
	for _, v := range s.Versions {
		if v.Available && len(out) < 20 {
			out = append(out, v.Version)
		}
	}
	return out
}

func summaryOf(s skillstypes.Skill) skillSummary {
	out := skillSummary{ID: s.ID, Name: s.Name, Title: s.Title, Description: s.Description, Tags: s.Tags, Path: s.Path, Author: s.Author,
		UpdatedAt: s.UpdatedAt, Downloads: s.Downloads, Versions: available(s)}
	if len(out.Versions) > 0 {
		out.Version = &out.Versions[0]
	}
	return out
}

func (t *tools) listSkills(ctx context.Context, _ *sdk.CallToolRequest, in listSkillsIn) (*sdk.CallToolResult, any, error) {
	q := skillsservices.ListInput{Query: in.Query, Tags: in.Tag, Limit: in.Limit, Cursor: in.Cursor, Recursive: true, Agent: true}
	if in.Folder != "" {
		id, err := t.folderID(ctx, in.Folder)
		if err != nil {
			return failed(err)
		}
		q.FolderID = &id
	}
	page, err := t.deps.Skills.Skills.ListSkills(ctx, t.p, q)
	if err != nil {
		return failed(err)
	}
	out := struct {
		Items      []skillSummary `json:"items"`
		NextCursor *string        `json:"next_cursor"`
	}{Items: []skillSummary{}, NextCursor: page.NextCursor}
	for _, s := range page.Items {
		if s.VersionsAvailable > 0 {
			out.Items = append(out.Items, summaryOf(s))
		}
	}
	return jsonResult(out, t.max)
}

func (t *tools) resolveSkill(ctx context.Context, ref string) (skillstypes.Skill, *sdk.CallToolResult) {
	s, candidates, err := t.deps.Skills.Skills.ResolveSkill(ctx, t.p, ref)
	if err == nil {
		return s, nil
	}
	if len(candidates) > 0 {
		var b strings.Builder
		b.WriteString("skill_ambiguous: several skills are named " + ref + "; pass one of:\n")
		for _, c := range candidates {
			fmt.Fprintf(&b, "- %s (id %s)\n", c.Path, c.ID)
		}
		return s, &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: b.String()}}}
	}
	res, _, _ := failed(err)
	return s, res
}

func (t *tools) versionMissing(s skillstypes.Skill, err error) (*sdk.CallToolResult, any, error) {
	if apperr.From(err).Kind != apperr.NotFound {
		return failed(err)
	}
	msg := "not_found: no such file or available version; available versions: " + strings.Join(available(s), ", ")
	return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: msg}}}, nil, nil
}

func (t *tools) getSkill(ctx context.Context, _ *sdk.CallToolRequest, in skillIn) (*sdk.CallToolResult, any, error) {
	s, res := t.resolveSkill(ctx, in.Skill)
	if res != nil {
		return res, nil, nil
	}
	sel, files, err := t.deps.Skills.Content.AgentContent(ctx, t.p, s.ID, in.Version)
	if err != nil {
		return t.versionMissing(s, err)
	}
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: skillText(sel, files, available(s), t.max)}}}, nil, nil
}

func skillText(sel skillsservices.Selected, files []skillsservices.ArchiveFile, versions []string, limit int) string {
	s, v := sel.Skill, sel.Version
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\nname: %s\nid: %s\npath: %s\nversion: %s\navailable versions: %s\nauthor: %s\ntags: %s\n\n%s\n\n",
		s.Title, s.Name, s.ID, s.Path, v.Version, strings.Join(versions, ", "), s.Author, strings.Join(s.Tags, ", "), s.Description)
	texts := map[string]string{}
	for _, f := range files {
		texts[f.Path] = string(f.Data)
	}
	fmt.Fprintf(&b, "## SKILL.md\n\n````markdown\n%s\n````\n\n## Files\n\n", texts[skillstypes.ManifestFile])
	for _, f := range sel.Files {
		fmt.Fprintf(&b, "- %s (%d bytes, %s)\n", f.Path, f.Size, f.Kind)
	}
	fmt.Fprintf(&b, "\n## Install\n\nCreate the folder `%s/` in the skills directory of your agent (Claude Code: `~/.claude/skills/%s/`) "+
		"and write every file below at its path inside it, byte for byte. Call get_guide with topic skills for details.\n", s.Name, s.Name)
	var rest []string
	for _, f := range sel.Files {
		text, ok := texts[f.Path]
		if f.Path == skillstypes.ManifestFile {
			continue
		}
		block := fmt.Sprintf("\n## %s\n\n````\n%s\n````\n", f.Path, text)
		if !ok || b.Len()+len(block) > limit-512 {
			rest = append(rest, f.Path)
			continue
		}
		b.WriteString(block)
	}
	if len(rest) > 0 {
		fmt.Fprintf(&b, "\n## Not included\n\nFetch these with get_skill_file (skill %s, version %s): %s\n", s.ID, v.Version, strings.Join(rest, ", "))
	}
	return b.String()
}

func (t *tools) getSkillFile(ctx context.Context, _ *sdk.CallToolRequest, in skillFileIn) (*sdk.CallToolResult, any, error) {
	s, res := t.resolveSkill(ctx, in.Skill)
	if res != nil {
		return res, nil, nil
	}
	f, data, err := t.deps.Skills.Content.AgentFile(ctx, t.p, s.ID, in.Path, in.Version)
	if err != nil {
		return t.versionMissing(s, err)
	}
	if f.Kind == skillstypes.KindMarkdown || f.Kind == skillstypes.KindText {
		return text(string(data), t.max), nil, nil
	}
	if base64.StdEncoding.EncodedLen(len(data)) > t.max {
		return failed(apperr.Invalid("too_large", "the file is larger than the result limit; download the archive over HTTP"))
	}
	uri := "svcr://skills/" + s.ID.String() + "/" + f.Path
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.EmbeddedResource{Resource: &sdk.ResourceContents{URI: uri, MIMEType: f.ContentType,
		Blob: data}}}}, nil, nil
}

func (t *tools) readSkillResource(ctx context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
	uri := req.Params.URI
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "svcr" || u.Host != "skills" {
		return nil, sdk.ResourceNotFoundError(uri)
	}
	id, path, ok := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/")
	skill, err := uuid.Parse(id)
	if !ok || err != nil || path == "" {
		return nil, sdk.ResourceNotFoundError(uri)
	}
	f, data, err := t.deps.Skills.Content.AgentFile(ctx, t.p, skill, path, u.Query().Get("version"))
	if err != nil {
		return nil, sdk.ResourceNotFoundError(uri)
	}
	out := &sdk.ResourceContents{URI: uri, MIMEType: f.ContentType}
	if f.Kind == skillstypes.KindMarkdown || f.Kind == skillstypes.KindText {
		out.Text, out.MIMEType = string(data), mimeOf(path)
	} else {
		out.Blob = data
	}
	return &sdk.ReadResourceResult{Contents: []*sdk.ResourceContents{out}}, nil
}
