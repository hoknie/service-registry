package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/catalog/repositories"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
)

type IssuedKey struct {
	Key    catalogtypes.ProjectKey
	Secret string
}

func (s *ProjectKeysService) authorizeKeys(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	sc, err := s.access.Authorize(ctx, p, catalogtypes.PermKeys, id)
	if err != nil {
		return err
	}
	if sc.Node.Kind != catalogtypes.KindProject {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

func (s *ProjectKeysService) issueKey(ctx context.Context, projectID uuid.UUID, graceSecs uint64) (IssuedKey, error) {
	fresh, err := auth.GenerateProjectKey()
	if err != nil {
		return IssuedKey{}, apperr.Internalf("no randomness: %v", err)
	}
	key, err := s.projectKeyRepository.Rotate(ctx, catalogtypes.NewKey{
		ID: uuid.Must(uuid.NewV7()), ProjectID: projectID, Prefix: fresh.Prefix, Hash: fresh.Hash,
	}, graceSecs)
	if err != nil {
		return IssuedKey{}, apperr.Wrap(err)
	}
	return IssuedKey{Key: key, Secret: fresh.Secret}, nil
}

func (s *ProjectKeysService) ListKeys(ctx context.Context, p accesstypes.Principal, id uuid.UUID) ([]catalogtypes.ProjectKey, error) {
	if err := s.authorizeKeys(ctx, p, id); err != nil {
		return nil, err
	}
	keys, err := s.projectKeyRepository.List(ctx, id)
	return keys, apperr.Wrap(err)
}

func (s *ProjectKeysService) RotateKey(ctx context.Context, p accesstypes.Principal, id uuid.UUID, graceSecs *int64) (IssuedKey, error) {
	if err := s.authorizeKeys(ctx, p, id); err != nil {
		return IssuedKey{}, err
	}
	grace, err := catalogtypes.ValidateGrace(graceSecs)
	if err != nil {
		return IssuedKey{}, apperr.Wrap(err)
	}
	return s.issueKey(ctx, id, grace)
}

func (s *ProjectKeysService) RevokeKey(ctx context.Context, p accesstypes.Principal, id, keyID uuid.UUID) error {
	if err := s.authorizeKeys(ctx, p, id); err != nil {
		return err
	}
	return apperr.Wrap(s.projectKeyRepository.Revoke(ctx, id, keyID))
}

func (s *ProjectKeysService) VerifyProjectKey(ctx context.Context, projectID uuid.UUID, presented string) (*uuid.UUID, error) {
	hash, ok := auth.ParseProjectKey(presented)
	if !ok {
		return nil, nil
	}
	return s.projectKeyRepository.Verify(ctx, projectID, hash)
}

type ProjectKeysService struct {
	projectKeyRepository *repositories.ProjectKeyRepository
	access               *AccessService
}
