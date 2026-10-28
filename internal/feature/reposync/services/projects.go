package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	reposyncports "svc-registry/internal/feature/reposync/ports"
	"svc-registry/internal/feature/reposync/repositories"
	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/apperr"
)

func (s *ProjectsService) ProjectRepository(ctx context.Context, id uuid.UUID) (*reposynctypes.Repository, error) {
	r, err := s.repoRepository.Find(ctx, id)
	if err != nil || r == nil {
		return nil, apperr.Wrap(err)
	}
	return new(r.FromHosting()), nil
}

func (s *ProjectsService) ProjectClient(ctx context.Context, id uuid.UUID) (reposynctypes.RemoteRepo, reposyncports.ClientPort, string) {
	r, err := s.repoRepository.Find(ctx, id)
	if err != nil || r == nil {
		return reposynctypes.RemoteRepo{}, nil, "not_found"
	}
	conn, err := s.connectionRepository.Find(ctx, r.ConnectionID)
	if err != nil || conn == nil {
		return reposynctypes.RemoteRepo{}, nil, "credentials_unavailable"
	}
	client, err := s.sync.clientFor(ctx, *conn)
	if err != nil {
		if code := reposynctypes.FailureCode(err); code != "" {
			return reposynctypes.RemoteRepo{}, nil, code
		}
		return reposynctypes.RemoteRepo{}, nil, "credentials_unavailable"
	}
	repo := reposynctypes.RemoteRepo{ExternalID: r.ExternalID, FullPath: r.FullPath}
	if r.DefaultBranch != nil {
		repo.DefaultBranch = *r.DefaultBranch
	}
	return repo, client, ""
}

func (s *ProjectsService) ProjectReadme(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (reposynctypes.Readme, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, id); err != nil {
		return reposynctypes.Readme{}, err
	}
	r, err := s.repoRepository.Readme(ctx, id)
	if err != nil {
		return reposynctypes.Readme{}, apperr.Wrap(err)
	}
	if r == nil {
		return reposynctypes.Readme{}, apperr.New(apperr.NotFound)
	}
	return *r, nil
}

type ProjectsService struct {
	catalog              *catalogservices.CatalogServices
	connectionRepository *repositories.ConnectionRepository
	repoRepository       *repositories.RepoRepository
	sync                 *SyncService
}
