package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
)

type IssuedKey struct {
	Key    catalog.ProjectKey
	Secret string
}

func (s *Service) authorizeKeys(ctx context.Context, p access.Principal, id uuid.UUID) error {
	sc, err := s.Authorize(ctx, p, catalog.PermKeys, id)
	if err != nil {
		return err
	}
	if sc.Node.Kind != catalog.KindProject {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

func (s *Service) issueKey(ctx context.Context, projectID uuid.UUID, graceSecs uint64) (IssuedKey, error) {
	fresh, err := auth.GenerateProjectKey()
	if err != nil {
		return IssuedKey{}, apperr.Internalf("no randomness: %v", err)
	}
	key, err := s.projectKeys.Rotate(ctx, catalog.NewKey{
		ID: uuid.Must(uuid.NewV7()), ProjectID: projectID, Prefix: fresh.Prefix, Hash: fresh.Hash,
	}, graceSecs)
	if err != nil {
		return IssuedKey{}, apperr.Wrap(err)
	}
	return IssuedKey{Key: key, Secret: fresh.Secret}, nil
}

func (s *Service) ListKeys(ctx context.Context, p access.Principal, id uuid.UUID) ([]catalog.ProjectKey, error) {
	if err := s.authorizeKeys(ctx, p, id); err != nil {
		return nil, err
	}
	keys, err := s.projectKeys.List(ctx, id)
	return keys, apperr.Wrap(err)
}

func (s *Service) RotateKey(ctx context.Context, p access.Principal, id uuid.UUID, graceSecs *int64) (IssuedKey, error) {
	if err := s.authorizeKeys(ctx, p, id); err != nil {
		return IssuedKey{}, err
	}
	grace, err := catalog.ValidateGrace(graceSecs)
	if err != nil {
		return IssuedKey{}, apperr.Wrap(err)
	}
	return s.issueKey(ctx, id, grace)
}

func (s *Service) RevokeKey(ctx context.Context, p access.Principal, id, keyID uuid.UUID) error {
	if err := s.authorizeKeys(ctx, p, id); err != nil {
		return err
	}
	return apperr.Wrap(s.projectKeys.Revoke(ctx, id, keyID))
}
