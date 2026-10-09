package service

import (
	"context"

	"svc-registry/internal/apperr"
	"svc-registry/internal/deploy"
)

func ListEnvironmentDirectory(ctx context.Context, state *State, _ Principal) ([]deploy.Environment, error) {
	items, err := state.Environments.List(ctx)
	return items, apperr.Wrap(err)
}

func CreateEnvironment(ctx context.Context, state *State, p Principal, in deploy.CreateEnvironment) (deploy.Environment, error) {
	if err := RequireSuperadmin(p); err != nil {
		return deploy.Environment{}, err
	}
	e, err := deploy.ValidateNewEnvironment(in)
	if err != nil {
		return deploy.Environment{}, apperr.Wrap(err)
	}
	created, err := state.Environments.Insert(ctx, e)
	return created, apperr.Wrap(err)
}

func UpdateEnvironment(ctx context.Context, state *State, p Principal, rawKey string, in deploy.UpdateEnvironment) (deploy.Environment, error) {
	if err := RequireSuperadmin(p); err != nil {
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
	e, err := state.Environments.Update(ctx, key, changes)
	if err != nil {
		return deploy.Environment{}, apperr.Wrap(err)
	}
	if e == nil {
		return deploy.Environment{}, apperr.New(apperr.NotFound)
	}
	return *e, nil
}

func DeleteEnvironment(ctx context.Context, state *State, p Principal, rawKey string) error {
	if err := RequireSuperadmin(p); err != nil {
		return err
	}
	key, err := deploy.EnvironmentKey(rawKey)
	if err != nil {
		return apperr.New(apperr.NotFound)
	}
	return apperr.Wrap(state.Environments.Delete(ctx, key))
}
