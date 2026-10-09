package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/auth"
	"svc-registry/internal/catalog"
)

type IssuedKey struct {
	Key    catalog.ProjectKey
	Secret string
}

func authorizeKeys(ctx context.Context, state *State, p Principal, id uuid.UUID) error {
	s, err := Authorize(ctx, state, p, catalog.PermKeys, id)
	if err != nil {
		return err
	}
	if s.Node.Kind != catalog.KindProject {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

func issueKey(ctx context.Context, state *State, projectID uuid.UUID, graceSecs uint64) (IssuedKey, error) {
	fresh, err := auth.GenerateProjectKey()
	if err != nil {
		return IssuedKey{}, apperr.Internalf("no randomness: %v", err)
	}
	key, err := state.ProjectKeys.Rotate(ctx, catalog.NewKey{
		ID: uuid.Must(uuid.NewV7()), ProjectID: projectID, Prefix: fresh.Prefix, Hash: fresh.Hash,
	}, graceSecs)
	if err != nil {
		return IssuedKey{}, apperr.Wrap(err)
	}
	return IssuedKey{Key: key, Secret: fresh.Secret}, nil
}

func ListKeys(ctx context.Context, state *State, p Principal, id uuid.UUID) ([]catalog.ProjectKey, error) {
	if err := authorizeKeys(ctx, state, p, id); err != nil {
		return nil, err
	}
	keys, err := state.ProjectKeys.List(ctx, id)
	return keys, apperr.Wrap(err)
}

func RotateKey(ctx context.Context, state *State, p Principal, id uuid.UUID, graceSecs *int64) (IssuedKey, error) {
	if err := authorizeKeys(ctx, state, p, id); err != nil {
		return IssuedKey{}, err
	}
	grace, err := catalog.ValidateGrace(graceSecs)
	if err != nil {
		return IssuedKey{}, apperr.Wrap(err)
	}
	return issueKey(ctx, state, id, grace)
}

func RevokeKey(ctx context.Context, state *State, p Principal, id, keyID uuid.UUID) error {
	if err := authorizeKeys(ctx, state, p, id); err != nil {
		return err
	}
	return apperr.Wrap(state.ProjectKeys.Revoke(ctx, id, keyID))
}
