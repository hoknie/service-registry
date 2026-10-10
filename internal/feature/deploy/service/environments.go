package service

import (
	"context"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/deploy"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) ListEnvironmentDirectory(ctx context.Context, _ access.Principal) ([]deploy.Environment, error) {
	items, err := s.environments.List(ctx)
	return items, apperr.Wrap(err)
}

func (s *Service) CreateEnvironment(ctx context.Context, p access.Principal, in deploy.CreateEnvironment) (deploy.Environment, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return deploy.Environment{}, err
	}
	e, err := deploy.ValidateNewEnvironment(in)
	if err != nil {
		return deploy.Environment{}, apperr.Wrap(err)
	}
	created, err := s.environments.Insert(ctx, e)
	return created, apperr.Wrap(err)
}

func (s *Service) UpdateEnvironment(ctx context.Context, p access.Principal, rawKey string, in deploy.UpdateEnvironment) (deploy.Environment, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return deploy.Environment{}, err
	}
	key, err := deploy.EnvironmentKey(rawKey)
	if err != nil {
		return deploy.Environment{}, apperr.New(apperr.NotFound)
	}
	changes, err := deploy.ValidateEnvironmentChanges(in)
	if err != nil {
		return deploy.Environment{}, apperr.Wrap(err)
	}
	e, err := s.environments.Update(ctx, key, changes)
	if err != nil {
		return deploy.Environment{}, apperr.Wrap(err)
	}
	if e == nil {
		return deploy.Environment{}, apperr.New(apperr.NotFound)
	}
	return *e, nil
}

func (s *Service) DeleteEnvironment(ctx context.Context, p access.Principal, rawKey string) error {
	if err := access.RequireSuperadmin(p); err != nil {
		return err
	}
	key, err := deploy.EnvironmentKey(rawKey)
	if err != nil {
		return apperr.New(apperr.NotFound)
	}
	return apperr.Wrap(s.environments.Delete(ctx, key))
}
