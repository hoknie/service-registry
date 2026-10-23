package services

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
)

const (
	docSpaceChildren  = 500
	docSpacePageLimit = 200
	docSpaceTreeDepth = 10
)

type DocSpaceNode struct {
	ID          uuid.UUID
	Kind        catalogtypes.NodeKind
	Slug        string
	Name        string
	Description *string
	Readable    bool
	HasDocs     bool
}

type DocSpaceBranch struct {
	Name        string
	CollectedAt string
}

type DocSpaceDocs struct {
	Source        knowledgetypes.SourceKind
	DefaultBranch string
	Branches      []DocSpaceBranch
}

type DocSpace struct {
	Node      *DocSpaceNode
	Ancestors []DocSpaceNode
	Children  []DocSpaceNode
	Docs      *DocSpaceDocs
	DocPath   *string
}

func docSegments(raw string) ([]string, bool) {
	trimmed := strings.Trim(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return nil, true
	}
	parts := strings.Split(trimmed, "/")
	out := make([]string, 0, len(parts))
	for _, s := range parts {
		if s == "" || strings.ContainsAny(s, "\\\x00") {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func (s *SpaceService) DocSpace(ctx context.Context, p accesstypes.Principal, rawPath string) (DocSpace, error) {
	segments, ok := docSegments(rawPath)
	if !ok {
		return DocSpace{}, apperr.New(apperr.NotFound)
	}
	var cur *catalogtypes.Node
	var parent *uuid.UUID
	var docPath *string
	for i, seg := range segments {
		if cur != nil && cur.Kind == catalogtypes.KindProject {
			docPath = new(strings.Join(segments[i:], "/"))
			break
		}
		n, err := s.catalog.System.FindChild(ctx, parent, strings.ToLower(seg))
		if err != nil {
			return DocSpace{}, apperr.Wrap(err)
		}
		if n == nil {
			return DocSpace{}, apperr.New(apperr.NotFound)
		}
		cur, parent = n, &n.ID
	}
	out := DocSpace{Ancestors: []DocSpaceNode{}, Children: []DocSpaceNode{}, DocPath: docPath}
	if cur != nil {
		sc, err := s.catalog.Access.Scope(ctx, p, cur.ID)
		if err != nil {
			return DocSpace{}, apperr.New(apperr.NotFound)
		}
		readable := sc.Visibility == catalogtypes.VisibilityRead
		node := spaceNode(sc.Node, readable)
		if readable {
			node.Description = new(sc.Node.Description)
		}
		out.Node = &node
		for _, a := range sc.Path {
			out.Ancestors = append(out.Ancestors, spaceNode(a.Node, a.Readable))
		}
		if sc.Node.Kind == catalogtypes.KindProject {
			docs, err := s.spaceDocs(ctx, p, sc.Node.ID)
			if err != nil {
				return DocSpace{}, err
			}
			out.Docs = &docs
			with, err := s.snapshotRepository.ProjectsWithDocs(ctx, []uuid.UUID{sc.Node.ID})
			if err != nil {
				return DocSpace{}, apperr.Wrap(err)
			}
			out.Node.HasDocs = with[sc.Node.ID]
			return out, nil
		}
	}
	children, err := s.spaceChildren(ctx, p, parent)
	if err != nil {
		return DocSpace{}, err
	}
	out.Children = children
	return out, nil
}

func spaceNode(n catalogtypes.Node, readable bool) DocSpaceNode {
	return DocSpaceNode{ID: n.ID, Kind: n.Kind, Slug: n.Slug, Name: n.Name, Readable: readable}
}

func (s *SpaceService) spaceChildren(ctx context.Context, p accesstypes.Principal, parent *uuid.UUID) ([]DocSpaceNode, error) {
	out := []DocSpaceNode{}
	for offset := int64(0); len(out) < docSpaceChildren; offset += docSpacePageLimit {
		limit := int64(min(docSpacePageLimit, docSpaceChildren-len(out)))
		page, err := s.catalog.Nodes.Children(ctx, p, parent, accesstypes.PageQuery{Limit: &limit, Offset: &offset})
		if err != nil {
			return nil, err
		}
		for _, w := range page.Items {
			out = append(out, spaceNode(w.Node, w.Readable))
		}
		if len(page.Items) < int(limit) || uint64(offset)+uint64(len(page.Items)) >= page.Total {
			break
		}
	}
	if len(out) == 0 {
		return out, nil
	}
	tree, _, err := s.catalog.Nodes.Tree(ctx, p, parent, new(int64(docSpaceTreeDepth)))
	if err != nil {
		return nil, err
	}
	parents := make(map[uuid.UUID]uuid.UUID, len(tree))
	top := make(map[uuid.UUID]bool, len(out))
	for _, c := range out {
		top[c.ID] = true
	}
	var projects []uuid.UUID
	for _, w := range tree {
		if w.Node.ParentID != nil {
			parents[w.Node.ID] = *w.Node.ParentID
		}
		if w.Node.Kind == catalogtypes.KindProject && w.Readable {
			projects = append(projects, w.Node.ID)
		}
	}
	with, err := s.snapshotRepository.ProjectsWithDocs(ctx, projects)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	has := map[uuid.UUID]bool{}
	for id := range with {
		for cur, ok := id, true; ok; cur, ok = parents[cur] {
			if top[cur] {
				has[cur] = true
				break
			}
		}
	}
	for i := range out {
		out[i].HasDocs = has[out[i].ID]
	}
	return out, nil
}

func (s *SpaceService) spaceDocs(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (DocSpaceDocs, error) {
	kp, err := s.documents.knowledgeProject(ctx, p, catalogtypes.PermRead, id)
	if err != nil {
		return DocSpaceDocs{}, err
	}
	d, err := s.documents.documentationOf(ctx, kp)
	if err != nil {
		return DocSpaceDocs{}, err
	}
	overview, err := s.documents.GetKnowledge(ctx, p, id)
	if err != nil {
		return DocSpaceDocs{}, err
	}
	out := DocSpaceDocs{Source: d.kind(), DefaultBranch: d.Default, Branches: []DocSpaceBranch{}}
	for _, b := range overview.Branches {
		if b.Snapshot != nil {
			out.Branches = append(out.Branches, DocSpaceBranch{Name: b.Name, CollectedAt: b.Snapshot.CollectedAt})
		}
	}
	return out, nil
}

type SpaceService struct {
	catalog            *catalogservices.CatalogServices
	snapshotRepository *repositories.SnapshotRepository
	documents          *DocumentsService
}

type SpecItem struct {
	Capability, Path, Purpose string
	Requirements              []string
}

type ChangeItem struct {
	ID, Path  string
	Archived  bool
	Artifacts []string
}

type ADRItem struct {
	Number                          int
	Title, Status, Supersedes, Path string
}

type OpenSpec struct {
	Specs   []SpecItem
	Changes []ChangeItem
	ADRs    []ADRItem
}

func (s *DocumentsService) KnowledgeOpenSpec(ctx context.Context, p accesstypes.Principal, id uuid.UUID, branch *string) (OpenSpec, error) {
	out := OpenSpec{Specs: []SpecItem{}, Changes: []ChangeItem{}, ADRs: []ADRItem{}}
	_, files, err := s.KnowledgeFiles(ctx, p, id, KnowledgeRef{Branch: branch})
	if apperr.From(err) != nil && apperr.From(err).Kind == apperr.NotFound {
		if _, err := s.knowledgeProject(ctx, p, catalogtypes.PermRead, id); err != nil {
			return OpenSpec{}, err
		}
		return out, nil
	}
	if err != nil {
		return OpenSpec{}, err
	}
	changes := map[string]*ChangeItem{}
	var order []string
	for _, f := range files {
		switch f.Kind {
		case knowledgetypes.KindSpec:
			reqs := f.Meta.Requirements
			if reqs == nil {
				reqs = []string{}
			}
			out.Specs = append(out.Specs, SpecItem{Capability: f.Meta.Capability, Path: f.Path, Purpose: f.Meta.Purpose, Requirements: reqs})
		case knowledgetypes.KindChange:
			key := f.Meta.Change
			if f.Meta.Archived {
				key = "archive/" + key
			}
			c := changes[key]
			if c == nil {
				prefix := "openspec/changes/" + key
				c = &ChangeItem{ID: f.Meta.Change, Path: prefix, Archived: f.Meta.Archived, Artifacts: []string{}}
				changes[key] = c
				order = append(order, key)
			}
			c.Artifacts = append(c.Artifacts, strings.TrimPrefix(f.Path, c.Path+"/"))
		case knowledgetypes.KindADR:
			out.ADRs = append(out.ADRs, ADRItem{Number: f.Meta.Number, Title: f.Meta.Title, Status: f.Meta.Status,
				Supersedes: f.Meta.Supersedes, Path: f.Path})
		}
	}
	slices.SortFunc(out.Specs, func(a, b SpecItem) int { return strings.Compare(a.Capability, b.Capability) })
	for _, k := range order {
		out.Changes = append(out.Changes, *changes[k])
	}
	slices.SortStableFunc(out.Changes, func(a, b ChangeItem) int {
		switch {
		case a.Archived != b.Archived && !a.Archived:
			return -1
		case a.Archived != b.Archived:
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	slices.SortFunc(out.ADRs, func(a, b ADRItem) int { return a.Number - b.Number })
	return out, nil
}
