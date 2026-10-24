package services

import (
	"context"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/deploy/repositories"
	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/apperr"
)

func (s *EnvironmentsService) ListEnvironmentDirectory(ctx context.Context, _ accesstypes.Principal) ([]deploytypes.Environment, error) {
	items, err := s.environmentRepository.List(ctx)
	return items, apperr.Wrap(err)
}

func (s *EnvironmentsService) CreateEnvironment(ctx context.Context, p accesstypes.Principal, in deploytypes.CreateEnvironment) (deploytypes.Environment, error) {
	if err := accesstypes.RequireRight(p, accesstypes.SectionEnvironments, accesstypes.ActionCreate); err != nil {
		return deploytypes.Environment{}, err
	}
	e, err := deploytypes.ValidateNewEnvironment(in)
	if err != nil {
		return deploytypes.Environment{}, apperr.Wrap(err)
	}
	created, err := s.environmentRepository.Insert(ctx, e)
	return created, apperr.Wrap(err)
}

func (s *EnvironmentsService) UpdateEnvironment(ctx context.Context, p accesstypes.Principal, rawKey string, in deploytypes.UpdateEnvironment) (deploytypes.Environment, error) {
	if err := accesstypes.RequireRight(p, accesstypes.SectionEnvironments, accesstypes.ActionUpdate); err != nil {
		return deploytypes.Environment{}, err
	}
	key, err := deploytypes.EnvironmentKey(rawKey)
	if err != nil {
		return deploytypes.Environment{}, apperr.New(apperr.NotFound)
	}
	changes, err := deploytypes.ValidateEnvironmentChanges(in)
	if err != nil {
		return deploytypes.Environment{}, apperr.Wrap(err)
	}
	e, err := s.environmentRepository.Update(ctx, key, changes)
	if err != nil {
		return deploytypes.Environment{}, apperr.Wrap(err)
	}
	if e == nil {
		return deploytypes.Environment{}, apperr.New(apperr.NotFound)
	}
	return *e, nil
}

func (s *EnvironmentsService) DeleteEnvironment(ctx context.Context, p accesstypes.Principal, rawKey string) error {
	if err := accesstypes.RequireRight(p, accesstypes.SectionEnvironments, accesstypes.ActionDelete); err != nil {
		return err
	}
	key, err := deploytypes.EnvironmentKey(rawKey)
	if err != nil {
		return apperr.New(apperr.NotFound)
	}
	return apperr.Wrap(s.environmentRepository.Delete(ctx, key))
}

type EnvironmentsService struct {
	environmentRepository *repositories.EnvironmentRepository
}
