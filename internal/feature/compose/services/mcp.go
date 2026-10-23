package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	forgeservices "svc-registry/internal/feature/forge/services"
	forgetypes "svc-registry/internal/feature/forge/types"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	ownershipservices "svc-registry/internal/feature/ownership/services"
	ownershiptypes "svc-registry/internal/feature/ownership/types"
	"svc-registry/internal/platform/apperr"
)

type ProjectOverview struct {
	Repository    *forgetypes.Repository
	Documentation knowledgeservices.KnowledgeOverview
	Owners        ownershiptypes.OwnerSet
}

type DocResource struct {
	Project catalogtypes.Node
	File    knowledgetypes.FileInfo
}

func (s *MCPService) ProjectOverview(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (ProjectOverview, error) {
	repo, err := s.forge.Projects.ProjectRepository(ctx, id)
	if err != nil {
		return ProjectOverview{}, apperr.Wrap(err)
	}
	docs, err := s.knowledge.Documents.GetKnowledge(ctx, p, id)
	if err != nil {
		return ProjectOverview{}, apperr.Wrap(err)
	}
	owners, err := s.ownership.Owners.NodeOwners(ctx, p, id)
	if err != nil {
		return ProjectOverview{}, apperr.Wrap(err)
	}
	return ProjectOverview{Repository: repo, Documentation: docs, Owners: owners}, nil
}

func (s *MCPService) DocResources(ctx context.Context, p accesstypes.Principal) ([]DocResource, error) {
	nodes, _, err := s.catalog.Nodes.Tree(ctx, p, nil, new(int64(catalogtypes.TreeMaxDepth)))
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	var out []DocResource
	for _, w := range nodes {
		if !w.Readable || w.Node.Kind != catalogtypes.KindProject {
			continue
		}
		_, files, err := s.knowledge.Documents.KnowledgeFiles(ctx, p, w.Node.ID, knowledgeservices.KnowledgeRef{})
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.Skip == nil {
				out = append(out, DocResource{Project: w.Node, File: f})
			}
		}
	}
	return out, nil
}

type MCPService struct {
	catalog   *catalogservices.CatalogServices
	forge     *forgeservices.ForgeServices
	knowledge *knowledgeservices.KnowledgeServices
	ownership *ownershipservices.OwnershipServices
}
