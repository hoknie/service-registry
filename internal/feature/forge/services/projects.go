package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	forgeports "svc-registry/internal/feature/forge/ports"
	"svc-registry/internal/feature/forge/repositories"
	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/apperr"
)

func (s *ProjectsService) ProjectRepository(ctx context.Context, id uuid.UUID) (*forgetypes.Repository, error) {
	r, err := s.repoRepository.Find(ctx, id)
	return r, apperr.Wrap(err)
}

func (s *ProjectsService) ProjectClient(ctx context.Context, id uuid.UUID) (forgetypes.RemoteRepo, forgeports.ClientPort, string) {
	r, err := s.repoRepository.Find(ctx, id)
	if err != nil || r == nil {
		return forgetypes.RemoteRepo{}, nil, "not_found"
	}
	conn, err := s.connectionRepository.Find(ctx, r.ConnectionID)
	if err != nil || conn == nil {
		return forgetypes.RemoteRepo{}, nil, "credentials_unavailable"
	}
	client, err := s.sync.clientFor(ctx, *conn)
	if err != nil {
		if code := forgetypes.FailureCode(err); code != "" {
			return forgetypes.RemoteRepo{}, nil, code
		}
		return forgetypes.RemoteRepo{}, nil, "credentials_unavailable"
	}
	repo := forgetypes.RemoteRepo{ExternalID: r.ExternalID, FullPath: r.FullPath}
	if r.DefaultBranch != nil {
		repo.DefaultBranch = *r.DefaultBranch
	}
	return repo, client, ""
}

func (s *ProjectsService) ProjectReadme(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (forgetypes.Readme, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, id); err != nil {
		return forgetypes.Readme{}, err
	}
	r, err := s.repoRepository.Readme(ctx, id)
	if err != nil {
		return forgetypes.Readme{}, apperr.Wrap(err)
	}
	if r == nil {
		return forgetypes.Readme{}, apperr.New(apperr.NotFound)
	}
	return *r, nil
}

type ProjectsService struct {
	catalog              *catalogservices.CatalogServices
	connectionRepository *repositories.ConnectionRepository
	repoRepository       *repositories.RepoRepository
	sync                 *SyncService
}
