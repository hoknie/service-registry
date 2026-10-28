package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogports "svc-registry/internal/feature/catalog/ports"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	reposyncservices "svc-registry/internal/feature/reposync/services"
	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/apperr"
)

type ProjectCard struct {
	Scoped        catalogservices.Scoped
	Repository    *reposynctypes.Repository
	HasRepository bool
}

func (s *ProjectsService) ProjectCard(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (ProjectCard, error) {
	scoped, err := s.catalog.Nodes.GetNode(ctx, p, id)
	if err != nil {
		return ProjectCard{}, apperr.Wrap(err)
	}
	card := ProjectCard{Scoped: scoped}
	if scoped.Role == nil || scoped.Node.Kind != catalogtypes.KindProject {
		return card, nil
	}
	repo, err := s.projectRepository(ctx, id)
	if err != nil {
		return ProjectCard{}, apperr.Wrap(err)
	}
	card.Repository, card.HasRepository = repo, true
	return card, nil
}

func (s *ProjectsService) UnlinkProject(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (ProjectCard, error) {
	if err := s.repoSync.Unlink.UnlinkProject(ctx, p, id); err != nil {
		return ProjectCard{}, apperr.Wrap(err)
	}
	return s.ProjectCard(ctx, p, id)
}

func (s *ProjectsService) ProjectReadme(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (reposynctypes.Readme, error) {
	r, err := s.repoSync.Projects.ProjectReadme(ctx, p, id)
	if e := apperr.From(err); e != nil && e.Kind == apperr.NotFound {
		if _, aerr := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, id); aerr != nil {
			return reposynctypes.Readme{}, apperr.Wrap(aerr)
		}
		src, serr := s.knowledge.Sources.SourceReadme(ctx, id)
		if serr != nil {
			return reposynctypes.Readme{}, apperr.Wrap(serr)
		}
		if src != nil {
			r, err = *src, nil
		}
	}
	if err != nil {
		return reposynctypes.Readme{}, apperr.Wrap(err)
	}
	return r, nil
}

func (s *ProjectsService) CreateNode(ctx context.Context, p accesstypes.Principal, in catalogtypes.CreateNode, source *knowledgetypes.SourceInput) (catalogtypes.Node, *catalogservices.IssuedKey, error) {
	var factory catalogports.SourceFactoryPort
	if source != nil {
		factory = s.knowledge.Ports.SourceFor(*source)
	}
	node, key, err := s.catalog.Nodes.CreateNode(ctx, p, in, factory)
	if err != nil {
		return catalogtypes.Node{}, nil, apperr.Wrap(err)
	}
	return node, key, nil
}

func (s *ProjectsService) projectRepository(ctx context.Context, id uuid.UUID) (*reposynctypes.Repository, error) {
	repo, err := s.repoSync.Projects.ProjectRepository(ctx, id)
	if err != nil || repo != nil {
		return repo, err
	}
	return s.knowledge.Sources.SourceRepository(ctx, id)
}

type ProjectsService struct {
	catalog   *catalogservices.CatalogServices
	repoSync  *reposyncservices.RepoSyncServices
	knowledge *knowledgeservices.KnowledgeServices
}
