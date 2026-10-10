package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/forge"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) ProjectRepository(ctx context.Context, id uuid.UUID) (*forge.Repository, error) {
	r, err := s.repositories.Find(ctx, id)
	return r, apperr.Wrap(err)
}

func (s *Service) ProjectClient(ctx context.Context, id uuid.UUID) (forge.RemoteRepo, forge.Client, string) {
	r, err := s.repositories.Find(ctx, id)
	if err != nil || r == nil {
		return forge.RemoteRepo{}, nil, "not_found"
	}
	conn, err := s.connections.Find(ctx, r.ConnectionID)
	if err != nil || conn == nil {
		return forge.RemoteRepo{}, nil, "credentials_unavailable"
	}
	client, err := s.clientFor(ctx, *conn)
	if err != nil {
		if code := forge.FailureCode(err); code != "" {
			return forge.RemoteRepo{}, nil, code
		}
		return forge.RemoteRepo{}, nil, "credentials_unavailable"
	}
	repo := forge.RemoteRepo{ExternalID: r.ExternalID, FullPath: r.FullPath}
	if r.DefaultBranch != nil {
		repo.DefaultBranch = *r.DefaultBranch
	}
	return repo, client, ""
}
