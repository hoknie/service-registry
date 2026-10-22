package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/presentation/http/responses"
	"svc-registry/pkg/ptr"
)

func (t *tools) listDocs(ctx context.Context, _ *sdk.CallToolRequest, in docsIn) (*sdk.CallToolResult, any, error) {
	s, err := resolveProject(ctx, t.deps, t.p, in.Project)
	if err != nil {
		return failed(err)
	}
	snap, files, err := t.deps.Knowledge.Documents.KnowledgeFiles(ctx, t.p, s.Node.ID, knowledgeservices.KnowledgeRef{Branch: ptr.NonEmpty(in.Branch), Commit: ptr.NonEmpty(in.Commit)})
	if err != nil {
		return failed(err)
	}
	var kept []knowledgetypes.FileInfo
	for _, f := range files {
		if strings.HasPrefix(f.Path, in.Path) {
			kept = append(kept, f)
		}
	}
	return jsonResult(responses.KnowledgeFilesOf(snap, kept), t.max)
}

func (t *tools) docText(ctx context.Context, project uuid.UUID, ref knowledgeservices.KnowledgeRef, path string) (*sdk.CallToolResult, any, error) {
	snap, f, err := t.deps.Knowledge.Documents.KnowledgeFile(ctx, t.p, project, ref, path)
	if err != nil {
		return failed(err)
	}
	head := fmt.Sprintf("path: %s\nbranch: %s\ncommit: %s\ncollected_at: %s\n\n", f.Path, snap.Branch, snap.Commit, snap.CollectedAt)
	if f.Content == nil {
		reason := ""
		if f.Skip != nil {
			reason = string(*f.Skip)
		}
		return text(head+"[not stored: "+reason+"]", t.max), nil, nil
	}
	return text(head+*f.Content, t.max), nil, nil
}

func (t *tools) getDoc(ctx context.Context, _ *sdk.CallToolRequest, in docIn) (*sdk.CallToolResult, any, error) {
	s, err := resolveProject(ctx, t.deps, t.p, in.Project)
	if err != nil {
		return failed(err)
	}
	return t.docText(ctx, s.Node.ID, knowledgeservices.KnowledgeRef{Branch: ptr.NonEmpty(in.Branch), Commit: ptr.NonEmpty(in.Commit)}, in.Path)
}

func (t *tools) searchDocs(ctx context.Context, _ *sdk.CallToolRequest, in searchIn) (*sdk.CallToolResult, any, error) {
	q := knowledgetypes.SearchInput{Q: in.Query, Branch: ptr.NonEmpty(in.Branch), Path: in.Path, Mode: in.Mode}
	if in.Limit != 0 {
		q.Limit = &in.Limit
	}
	if in.Project != "" {
		s, err := resolveProject(ctx, t.deps, t.p, in.Project)
		if err != nil {
			return failed(err)
		}
		q.Project = &s.Node.ID
	}
	res, err := t.deps.Knowledge.Search.SearchKnowledge(ctx, t.p, q)
	if err != nil {
		return failed(err)
	}
	return jsonResult(responses.SearchOf(res), t.max)
}

func (t *tools) openSpec(ctx context.Context, project, branch string) (uuid.UUID, knowledgeservices.OpenSpec, error) {
	s, err := resolveProject(ctx, t.deps, t.p, project)
	if err != nil {
		return uuid.UUID{}, knowledgeservices.OpenSpec{}, err
	}
	o, err := t.deps.Knowledge.Documents.KnowledgeOpenSpec(ctx, t.p, s.Node.ID, ptr.NonEmpty(branch))
	return s.Node.ID, o, err
}

func (t *tools) listSpecs(ctx context.Context, _ *sdk.CallToolRequest, in openSpecIn) (*sdk.CallToolResult, any, error) {
	_, o, err := t.openSpec(ctx, in.Project, in.Branch)
	if err != nil {
		return failed(err)
	}
	return jsonResult(responses.SpecsOf(o), t.max)
}

func (t *tools) getSpec(ctx context.Context, _ *sdk.CallToolRequest, in specIn) (*sdk.CallToolResult, any, error) {
	id, o, err := t.openSpec(ctx, in.Project, in.Branch)
	if err != nil {
		return failed(err)
	}
	for _, s := range o.Specs {
		if s.Capability == strings.Trim(in.Capability, "/") {
			return t.docText(ctx, id, knowledgeservices.KnowledgeRef{Branch: ptr.NonEmpty(in.Branch)}, s.Path)
		}
	}
	return failed(apperr.New(apperr.NotFound))
}

func (t *tools) listChanges(ctx context.Context, _ *sdk.CallToolRequest, in changesIn) (*sdk.CallToolResult, any, error) {
	_, o, err := t.openSpec(ctx, in.Project, in.Branch)
	if err != nil {
		return failed(err)
	}
	if in.Archived != nil {
		var kept []knowledgeservices.ChangeItem
		for _, c := range o.Changes {
			if c.Archived == *in.Archived {
				kept = append(kept, c)
			}
		}
		o.Changes = kept
	}
	return jsonResult(responses.ChangesOf(o), t.max)
}

func (t *tools) listADRs(ctx context.Context, _ *sdk.CallToolRequest, in openSpecIn) (*sdk.CallToolResult, any, error) {
	_, o, err := t.openSpec(ctx, in.Project, in.Branch)
	if err != nil {
		return failed(err)
	}
	return jsonResult(responses.ADRsOf(o), t.max)
}

func (t *tools) getADR(ctx context.Context, _ *sdk.CallToolRequest, in adrIn) (*sdk.CallToolResult, any, error) {
	id, o, err := t.openSpec(ctx, in.Project, in.Branch)
	if err != nil {
		return failed(err)
	}
	for _, a := range o.ADRs {
		if a.Number == in.Number {
			return t.docText(ctx, id, knowledgeservices.KnowledgeRef{Branch: ptr.NonEmpty(in.Branch)}, a.Path)
		}
	}
	return failed(apperr.New(apperr.NotFound))
}
