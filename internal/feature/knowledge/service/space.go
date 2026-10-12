package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/platform/apperr"
)

const (
	docSpaceChildren  = 500
	docSpacePageLimit = 200
	docSpaceTreeDepth = 10
)

type DocSpaceNode struct {
	ID          uuid.UUID
	Kind        catalog.NodeKind
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
	Source        knowledge.SourceKind
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

func (s *Service) DocSpace(ctx context.Context, p access.Principal, rawPath string) (DocSpace, error) {
	segments, ok := docSegments(rawPath)
	if !ok {
		return DocSpace{}, apperr.New(apperr.NotFound)
	}
	var cur *catalog.Node
	var parent *uuid.UUID
	var docPath *string
	for i, seg := range segments {
		if cur != nil && cur.Kind == catalog.KindProject {
			rest := strings.Join(segments[i:], "/")
			docPath = &rest
			break
		}
		n, err := s.catalog.FindChild(ctx, parent, strings.ToLower(seg))
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
		sc, err := s.catalog.Scope(ctx, p, cur.ID)
		if err != nil {
			return DocSpace{}, apperr.New(apperr.NotFound)
		}
		readable := sc.Visibility == catalog.VisibilityRead
		node := spaceNode(sc.Node, readable)
		if readable {
			desc := sc.Node.Description
			node.Description = &desc
		}
		out.Node = &node
		for _, a := range sc.Path {
			out.Ancestors = append(out.Ancestors, spaceNode(a.Node, a.Readable))
		}
		if sc.Node.Kind == catalog.KindProject {
			docs, err := s.spaceDocs(ctx, p, sc.Node.ID)
			if err != nil {
				return DocSpace{}, err
			}
			out.Docs = &docs
			with, err := s.snapshots.ProjectsWithDocs(ctx, []uuid.UUID{sc.Node.ID})
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

func spaceNode(n catalog.Node, readable bool) DocSpaceNode {
	return DocSpaceNode{ID: n.ID, Kind: n.Kind, Slug: n.Slug, Name: n.Name, Readable: readable}
}

func (s *Service) spaceChildren(ctx context.Context, p access.Principal, parent *uuid.UUID) ([]DocSpaceNode, error) {
	out := []DocSpaceNode{}
	for offset := int64(0); len(out) < docSpaceChildren; offset += docSpacePageLimit {
		limit := int64(min(docSpacePageLimit, docSpaceChildren-len(out)))
		page, err := s.catalog.Children(ctx, p, parent, access.PageQuery{Limit: &limit, Offset: &offset})
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
	depth := int64(docSpaceTreeDepth)
	tree, _, err := s.catalog.Tree(ctx, p, parent, &depth)
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
		if w.Node.Kind == catalog.KindProject && w.Readable {
			projects = append(projects, w.Node.ID)
		}
	}
	with, err := s.snapshots.ProjectsWithDocs(ctx, projects)
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

func (s *Service) spaceDocs(ctx context.Context, p access.Principal, id uuid.UUID) (DocSpaceDocs, error) {
	kp, err := s.knowledgeProject(ctx, p, catalog.PermRead, id)
	if err != nil {
		return DocSpaceDocs{}, err
	}
	d, err := s.documentationOf(ctx, kp)
	if err != nil {
		return DocSpaceDocs{}, err
	}
	overview, err := s.GetKnowledge(ctx, p, id)
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
