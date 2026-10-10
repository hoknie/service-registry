package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/secretbox"
)

func (s *Service) ListNodeSecrets(ctx context.Context, p access.Principal, nodeID uuid.UUID) ([]catalog.Secret, error) {
	if _, err := s.Authorize(ctx, p, catalog.PermRead, nodeID); err != nil {
		return nil, err
	}
	items, err := s.secrets.Available(ctx, nodeID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.withUsage(ctx, items)
}

func (s *Service) CreateNodeSecret(ctx context.Context, p access.Principal, nodeID uuid.UUID, in catalog.SecretInput) (catalog.Secret, error) {
	sc, err := s.Authorize(ctx, p, catalog.PermAccess, nodeID)
	if err != nil {
		return catalog.Secret{}, err
	}
	if sc.Node.Kind == catalog.KindProject {
		return catalog.Secret{}, apperr.New(apperr.NotFound)
	}
	return s.createSecret(ctx, &nodeID, in)
}

func (s *Service) UpdateNodeSecret(ctx context.Context, p access.Principal, nodeID, id uuid.UUID, in catalog.SecretInput) (catalog.Secret, error) {
	if _, err := s.Authorize(ctx, p, catalog.PermAccess, nodeID); err != nil {
		return catalog.Secret{}, err
	}
	return s.updateSecret(ctx, &nodeID, id, in)
}

func (s *Service) DeleteNodeSecret(ctx context.Context, p access.Principal, nodeID, id uuid.UUID) error {
	if _, err := s.Authorize(ctx, p, catalog.PermAccess, nodeID); err != nil {
		return err
	}
	return s.deleteSecret(ctx, &nodeID, id)
}

func (s *Service) ListGlobalSecrets(ctx context.Context, p access.Principal) ([]catalog.Secret, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return nil, err
	}
	items, err := s.secrets.Global(ctx)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.withUsage(ctx, items)
}

func (s *Service) GetGlobalSecret(ctx context.Context, p access.Principal, id uuid.UUID) (catalog.Secret, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return catalog.Secret{}, err
	}
	found, err := s.secrets.Get(ctx, id)
	if err != nil {
		return catalog.Secret{}, apperr.Wrap(err)
	}
	if found == nil || found.NodeID != nil {
		return catalog.Secret{}, apperr.New(apperr.NotFound)
	}
	items, err := s.withUsage(ctx, []catalog.Secret{*found})
	if err != nil {
		return catalog.Secret{}, err
	}
	return items[0], nil
}

func (s *Service) CreateGlobalSecret(ctx context.Context, p access.Principal, in catalog.SecretInput) (catalog.Secret, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return catalog.Secret{}, err
	}
	return s.createSecret(ctx, nil, in)
}

func (s *Service) UpdateGlobalSecret(ctx context.Context, p access.Principal, id uuid.UUID, in catalog.SecretInput) (catalog.Secret, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return catalog.Secret{}, err
	}
	return s.updateSecret(ctx, nil, id, in)
}

func (s *Service) DeleteGlobalSecret(ctx context.Context, p access.Principal, id uuid.UUID) error {
	if err := access.RequireSuperadmin(p); err != nil {
		return err
	}
	return s.deleteSecret(ctx, nil, id)
}

func (s *Service) seal(id uuid.UUID, v catalog.SecretValue) (catalog.StoredSecret, error) {
	if v.Ref != nil {
		return catalog.StoredSecret{Ref: v.Ref}, nil
	}
	if s.box == nil || !s.box.CanEncrypt() {
		return catalog.StoredSecret{}, apperr.Wrap(catalog.ConflictSecretsKeyMissing)
	}
	enc, err := s.box.Seal(*v.Value, secretbox.AAD(catalog.SecretTable, id.String(), catalog.SecretColumnValue))
	if err != nil {
		return catalog.StoredSecret{}, apperr.Internalf("encrypt: %v", err)
	}
	fp := secretbox.Fingerprint(*v.Value)
	return catalog.StoredSecret{Enc: &enc, Fingerprint: &fp}, nil
}

func (s *Service) createSecret(ctx context.Context, nodeID *uuid.UUID, in catalog.SecretInput) (catalog.Secret, error) {
	v, err := catalog.ValidateSecret(in, true)
	if err != nil {
		return catalog.Secret{}, apperr.Wrap(err)
	}
	id := uuid.Must(uuid.NewV7())
	stored, err := s.seal(id, *v.Value)
	if err != nil {
		return catalog.Secret{}, err
	}
	n := catalog.NewSecret{ID: id, NodeID: nodeID, Name: *v.Name, Stored: stored}
	if v.Description != nil {
		n.Description = *v.Description
	}
	created, err := s.secrets.Insert(ctx, n)
	return created, apperr.Wrap(err)
}

func (s *Service) updateSecret(ctx context.Context, scope *uuid.UUID, id uuid.UUID, in catalog.SecretInput) (catalog.Secret, error) {
	v, err := catalog.ValidateSecret(in, false)
	if err != nil {
		return catalog.Secret{}, apperr.Wrap(err)
	}
	changes := catalog.SecretChanges{Name: v.Name, Description: v.Description}
	if v.Value != nil {
		stored, err := s.seal(id, *v.Value)
		if err != nil {
			return catalog.Secret{}, err
		}
		changes.Stored = &stored
	}
	updated, err := s.secrets.Update(ctx, id, scope, changes)
	if err != nil {
		return catalog.Secret{}, apperr.Wrap(err)
	}
	items, err := s.withUsage(ctx, []catalog.Secret{updated})
	if err != nil {
		return catalog.Secret{}, err
	}
	return items[0], nil
}

func (s *Service) deleteSecret(ctx context.Context, scope *uuid.UUID, id uuid.UUID) error {
	found, err := s.secrets.Get(ctx, id)
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
		return apperr.Wrap(catalog.ConflictSecretInUse)
	}
	return apperr.Wrap(s.secrets.Delete(ctx, id, scope))
}

func sameScope(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (s *Service) usage(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
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

func (s *Service) withUsage(ctx context.Context, items []catalog.Secret) ([]catalog.Secret, error) {
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

func (s *Service) SecretFor(ctx context.Context, nodeID *uuid.UUID, id uuid.UUID) (catalog.SecretRef, error) {
	found, err := s.secrets.Get(ctx, id)
	if err != nil {
		return catalog.SecretRef{}, apperr.Wrap(err)
	}
	if found == nil {
		return catalog.SecretRef{}, apperr.Wrap(catalog.InvalidSecretNotAvailable)
	}
	ref := catalog.SecretRef{ID: found.ID, Name: found.Name, From: found.From}
	if found.NodeID == nil {
		return ref, nil
	}
	if nodeID == nil {
		return catalog.SecretRef{}, apperr.Wrap(catalog.InvalidSecretNotAvailable)
	}
	available, err := s.secrets.Available(ctx, *nodeID)
	if err != nil {
		return catalog.SecretRef{}, apperr.Wrap(err)
	}
	for _, a := range available {
		if a.ID == id {
			return ref, nil
		}
	}
	return catalog.SecretRef{}, apperr.Wrap(catalog.InvalidSecretNotAvailable)
}

func (s *Service) SecretRefs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]catalog.SecretRef, error) {
	return s.secrets.Refs(ctx, ids)
}

func (s *Service) ResolveSecret(ctx context.Context, id uuid.UUID) (string, error) {
	stored, err := s.secrets.Stored(ctx, id)
	if err != nil {
		return "", err
	}
	if stored == nil {
		return "", catalog.ErrNotFound
	}
	if stored.Ref != nil {
		return config.ResolveSecretRef(*stored.Ref)
	}
	if s.box == nil {
		return "", catalog.ConflictSecretsKeyMissing
	}
	return s.box.Open(*stored.Enc, secretbox.AAD(catalog.SecretTable, id.String(), catalog.SecretColumnValue))
}

func (s *Service) PrepareSecretRotation(ctx context.Context) (func(context.Context) error, int, error) {
	rows, err := s.secrets.Sealed(ctx)
	if err != nil {
		return nil, 0, err
	}
	plain := make([]string, len(rows))
	for i, r := range rows {
		v, err := s.box.Open(r.Enc, secretbox.AAD(catalog.SecretTable, r.ID.String(), catalog.SecretColumnValue))
		if err != nil {
			return nil, 0, err
		}
		plain[i] = v
	}
	apply := func(ctx context.Context) error {
		for i, r := range rows {
			enc, err := s.box.Seal(plain[i], secretbox.AAD(catalog.SecretTable, r.ID.String(), catalog.SecretColumnValue))
			if err != nil {
				return err
			}
			if err := s.secrets.ReplaceValue(ctx, r.ID, enc); err != nil {
				return err
			}
		}
		return nil
	}
	return apply, len(rows), nil
}
