package services

import (
	"context"
	"strings"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/reposync/repositories"
	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/apperr"
)

type PreviewItem struct {
	FullPath string
	Action   string
	Code     *string
}

func (s *PreviewsService) PreviewConnection(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID) ([]PreviewItem, error) {
	c, err := s.connectionsService.authorizeConnection(ctx, p, catalogtypes.PermWrite, nodeID, id)
	if err != nil {
		return nil, err
	}
	client, err := s.sync.clientFor(ctx, c, s.sync.effective(ctx, c.NodeID).Timeout())
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	targets, orphans, err := s.sync.remoteState(ctx, c, client)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	groups, err := s.repoRepository.Groups(ctx, c.ID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	items := make([]PreviewItem, 0, len(targets)+len(orphans))
	for _, t := range targets {
		item, err := s.previewOne(ctx, c, groups, t)
		if err != nil {
			return nil, apperr.Wrap(err)
		}
		items = append(items, item)
	}
	for _, o := range orphans {
		items = append(items, PreviewItem{FullPath: o.FullPath, Action: "orphan"})
	}
	return items, nil
}

func (s *PreviewsService) previewOne(ctx context.Context, c reposynctypes.Connection, groups map[string]uuid.UUID, t reposynctypes.Target) (PreviewItem, error) {
	r := t.Remote
	item := PreviewItem{FullPath: r.FullPath}
	if r.TooLong() {
		item.FullPath = reposynctypes.ProblemPath(r.FullPath)
		return skipped(item, reposynctypes.CodeValueTooLong), nil
	}
	parent := &c.NodeID
	segs := reposynctypes.Subgroups(c.OwnerPath, r.FullPath, c.MirrorSubgroups)
	for i := range segs {
		id, ok := groups[strings.ToLower(reposynctypes.GroupPath(c.OwnerPath, segs, i+1))]
		if !ok {
			clash, err := s.catalog.System.FindChild(ctx, parent, reposynctypes.Slugify(segs[i]))
			if err != nil {
				return item, err
			}
			if clash != nil {
				return skipped(item, catalogtypes.ConflictSlugTaken.Code), nil
			}
			parent = nil
			break
		}
		parent = &id
	}
	slug := reposynctypes.Slugify(r.Name)
	if t.Link == nil {
		item.Action = "create"
		if parent != nil {
			clash, err := s.catalog.System.FindChild(ctx, parent, slug)
			if err != nil {
				return item, err
			}
			if clash != nil {
				return skipped(item, catalogtypes.ConflictSlugTaken.Code), nil
			}
		}
		return item, nil
	}
	node, err := s.catalog.System.FindNode(ctx, t.Link.ProjectID)
	if err != nil {
		return item, err
	}
	item.Action = "update"
	if node != nil && (parent == nil || node.ParentID == nil || *node.ParentID != *parent ||
		(!strings.EqualFold(t.Link.FullPath, r.FullPath) && node.Slug != slug)) {
		item.Action = "move"
	}
	return item, nil
}

func skipped(item PreviewItem, code string) PreviewItem {
	item.Action = "skip"
	item.Code = &code
	return item
}

type PreviewsService struct {
	catalog            *catalogservices.CatalogServices
	repoRepository     *repositories.RepoRepository
	connectionsService *ConnectionsService
	sync               *SyncService
}
