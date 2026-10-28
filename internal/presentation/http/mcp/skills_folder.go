package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"svc-registry/internal/platform/apperr"
)

const maxFolderNames = 200

type skillFolderIn struct {
	Folder string `json:"folder" jsonschema:"Skill folder id or path of slugs (examples/apollo)"`
}

func (t *tools) folderID(ctx context.Context, ref string) (uuid.UUID, error) {
	ref = strings.Trim(strings.TrimSpace(ref), "/")
	if id, err := uuid.Parse(ref); err == nil {
		return id, nil
	}
	var parent *uuid.UUID
	for slug := range strings.SplitSeq(ref, "/") {
		level, err := t.deps.Skills.Folders.ListFolders(ctx, t.p, parent)
		if err != nil {
			return uuid.Nil, err
		}
		found := false
		for _, f := range level {
			if f.Slug == slug {
				parent, found = &f.ID, true
				break
			}
		}
		if !found {
			return uuid.Nil, apperr.New(apperr.NotFound)
		}
	}
	if parent == nil {
		return uuid.Nil, apperr.New(apperr.NotFound)
	}
	return *parent, nil
}

func (t *tools) getSkillFolder(ctx context.Context, _ *sdk.CallToolRequest, in skillFolderIn) (*sdk.CallToolResult, any, error) {
	id, err := t.folderID(ctx, in.Folder)
	if err != nil {
		return failed(err)
	}
	plan, err := t.deps.Skills.Archive.FolderPlan(ctx, t.p, id, false)
	if err != nil {
		return failed(err)
	}
	base := strings.TrimSuffix(t.deps.PublicURL, "/")
	if base == "" {
		base = "https://<registry>"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Skills of folder %s\n\n%d skills will be installed", plan.Folder.Name, len(plan.Skills))
	if plan.Skipped > 0 {
		fmt.Fprintf(&b, " (%d skipped: the same name appears twice)", plan.Skipped)
	}
	b.WriteString(".\n\n## Install\n\nThe token needs the `read` scope:\n\n```sh\n")
	fmt.Fprintf(&b, "curl -fsSL -H \"Authorization: Bearer $SVCR_TOKEN\" %s/api/v1/skills/folders/%s/archive -o %s.zip && unzip -o %s.zip -d ~/.claude/skills\n```\n\n",
		base, plan.Folder.ID, plan.Folder.Slug, plan.Folder.Slug)
	b.WriteString("## Skills\n\n")
	for i, sel := range plan.Skills {
		if i == maxFolderNames {
			fmt.Fprintf(&b, "- … and %d more\n", len(plan.Skills)-maxFolderNames)
			break
		}
		fmt.Fprintf(&b, "- `%s` — %s\n", sel.Skill.Name, sel.Skill.Path)
	}
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: b.String()}}}, nil, nil
}
