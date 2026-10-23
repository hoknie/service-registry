package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogports "svc-registry/internal/feature/catalog/ports"
	"svc-registry/internal/feature/catalog/repositories"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/secretbox"
)

func (s *SecretsService) ListNodeSecrets(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID) ([]catalogtypes.Secret, error) {
	if _, err := s.access.Authorize(ctx, p, catalogtypes.PermRead, nodeID); err != nil {
		return nil, err
	}
	items, err := s.secretRepository.Available(ctx, nodeID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.withUsage(ctx, items)
}

func (s *SecretsService) CreateNodeSecret(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID, in catalogtypes.SecretInput) (catalogtypes.Secret, error) {
	sc, err := s.access.Authorize(ctx, p, catalogtypes.PermAccess, nodeID)
	if err != nil {
		return catalogtypes.Secret{}, err
	}
	if sc.Node.Kind == catalogtypes.KindProject {
		return catalogtypes.Secret{}, apperr.New(apperr.NotFound)
	}
	return s.createSecret(ctx, &nodeID, in)
}

func (s *SecretsService) UpdateNodeSecret(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID, in catalogtypes.SecretInput) (catalogtypes.Secret, error) {
	if _, err := s.access.Authorize(ctx, p, catalogtypes.PermAccess, nodeID); err != nil {
		return catalogtypes.Secret{}, err
	}
	return s.updateSecret(ctx, &nodeID, id, in)
}

func (s *SecretsService) DeleteNodeSecret(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID) error {
	if _, err := s.access.Authorize(ctx, p, catalogtypes.PermAccess, nodeID); err != nil {
		return err
	}
	return s.deleteSecret(ctx, &nodeID, id)
}

func (s *SecretsService) ListGlobalSecrets(ctx context.Context, p accesstypes.Principal) ([]catalogtypes.Secret, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return nil, err
	}
	items, err := s.secretRepository.Global(ctx)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.withUsage(ctx, items)
}

func (s *SecretsService) GetGlobalSecret(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (catalogtypes.Secret, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return catalogtypes.Secret{}, err
	}
	found, err := s.secretRepository.Get(ctx, id)
	if err != nil {
		return catalogtypes.Secret{}, apperr.Wrap(err)
	}
	if found == nil || found.NodeID != nil {
		return catalogtypes.Secret{}, apperr.New(apperr.NotFound)
	}
	items, err := s.withUsage(ctx, []catalogtypes.Secret{*found})
	if err != nil {
		return catalogtypes.Secret{}, err
	}
	return items[0], nil
}

func (s *SecretsService) CreateGlobalSecret(ctx context.Context, p accesstypes.Principal, in catalogtypes.SecretInput) (catalogtypes.Secret, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return catalogtypes.Secret{}, err
	}
	return s.createSecret(ctx, nil, in)
}

func (s *SecretsService) UpdateGlobalSecret(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in catalogtypes.SecretInput) (catalogtypes.Secret, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return catalogtypes.Secret{}, err
	}
	return s.updateSecret(ctx, nil, id, in)
}

func (s *SecretsService) DeleteGlobalSecret(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return err
	}
	return s.deleteSecret(ctx, nil, id)
}

func (s *SecretsService) seal(id uuid.UUID, v catalogtypes.SecretValue) (catalogtypes.StoredSecret, error) {
	return sealSecret(s.box, id, v)
}

func sealSecret(box *secretbox.Box, id uuid.UUID, v catalogtypes.SecretValue) (catalogtypes.StoredSecret, error) {
	if v.Ref != nil {
		return catalogtypes.StoredSecret{Ref: v.Ref}, nil
	}
	if box == nil || !box.CanEncrypt() {
		return catalogtypes.StoredSecret{}, apperr.Wrap(catalogtypes.ConflictSecretsKeyMissing)
	}
	enc, err := box.Seal(*v.Value, secretbox.AAD(catalogtypes.SecretTable, id.String(), catalogtypes.SecretColumnValue))
	if err != nil {
		return catalogtypes.StoredSecret{}, apperr.Internalf("encrypt: %v", err)
	}
	return catalogtypes.StoredSecret{Enc: &enc, Fingerprint: new(secretbox.Fingerprint(*v.Value))}, nil
}

func (s *SecretsService) createSecret(ctx context.Context, nodeID *uuid.UUID, in catalogtypes.SecretInput) (catalogtypes.Secret, error) {
	v, err := catalogtypes.ValidateSecret(in, true)
	if err != nil {
		return catalogtypes.Secret{}, apperr.Wrap(err)
	}
	id := uuid.Must(uuid.NewV7())
	stored, err := s.seal(id, *v.Value)
	if err != nil {
		return catalogtypes.Secret{}, err
	}
	n := catalogtypes.NewSecret{ID: id, NodeID: nodeID, Name: *v.Name, Stored: stored}
	if v.Description != nil {
		n.Description = *v.Description
	}
	created, err := s.secretRepository.Insert(ctx, n)
	return created, apperr.Wrap(err)
}

func (s *SecretsService) updateSecret(ctx context.Context, scope *uuid.UUID, id uuid.UUID, in catalogtypes.SecretInput) (catalogtypes.Secret, error) {
	v, err := catalogtypes.ValidateSecret(in, false)
	if err != nil {
		return catalogtypes.Secret{}, apperr.Wrap(err)
	}
	changes := catalogtypes.SecretChanges{Name: v.Name, Description: v.Description}
	if v.Value != nil {
		stored, err := s.seal(id, *v.Value)
		if err != nil {
			return catalogtypes.Secret{}, err
		}
		changes.Stored = &stored
	}
	updated, err := s.secretRepository.Update(ctx, id, scope, changes)
	if err != nil {
		return catalogtypes.Secret{}, apperr.Wrap(err)
	}
	items, err := s.withUsage(ctx, []catalogtypes.Secret{updated})
	if err != nil {
		return catalogtypes.Secret{}, err
	}
	return items[0], nil
}

func (s *SecretsService) deleteSecret(ctx context.Context, scope *uuid.UUID, id uuid.UUID) error {
	found, err := s.secretRepository.Get(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	if found == nil || !sameScope(found.NodeID, scope) {
		return apperr.New(apperr.NotFound)
	}
	used, err := s.usage(ctx, []uuid.UUID{id})
	if err != nil {
		return err
	}
	if used[id] > 0 {
		return apperr.Wrap(catalogtypes.ConflictSecretInUse)
	}
	return apperr.Wrap(s.secretRepository.Delete(ctx, id, scope))
}

func sameScope(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (s *SecretsService) usage(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	out := map[uuid.UUID]int64{}
	for _, u := range s.secretUsers {
		found, err := u.SecretUsage(ctx, ids)
		if err != nil {
			return nil, apperr.Wrap(err)
		}
		for id, n := range found {
			out[id] += n
		}
	}
	return out, nil
}

func (s *SecretsService) withUsage(ctx context.Context, items []catalogtypes.Secret) ([]catalogtypes.Secret, error) {
	ids := make([]uuid.UUID, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	used, err := s.usage(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].UsedBy = used[items[i].ID]
	}
	return items, nil
}

type SecretsService struct {
	box              *secretbox.Box
	secretUsers      []catalogports.SecretUserPort
	secretRepository *repositories.SecretRepository
	access           *AccessService
}
