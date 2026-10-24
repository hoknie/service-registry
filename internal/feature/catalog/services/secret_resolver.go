package services

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog/repositories"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/secretbox"
)

func (s *SecretResolverService) SecretFor(ctx context.Context, nodeID *uuid.UUID, id uuid.UUID) (catalogtypes.SecretRef, error) {
	found, err := s.secretRepository.Get(ctx, id)
	if err != nil {
		return catalogtypes.SecretRef{}, apperr.Wrap(err)
	}
	if found == nil {
		return catalogtypes.SecretRef{}, apperr.Wrap(catalogtypes.InvalidSecretNotAvailable)
	}
	ref := catalogtypes.SecretRef{ID: found.ID, Name: found.Name, From: found.From}
	if found.NodeID == nil {
		return ref, nil
	}
	if nodeID == nil {
		return catalogtypes.SecretRef{}, apperr.Wrap(catalogtypes.InvalidSecretNotAvailable)
	}
	available, err := s.secretRepository.Available(ctx, *nodeID, true)
	if err != nil {
		return catalogtypes.SecretRef{}, apperr.Wrap(err)
	}
	for _, a := range available {
		if a.ID == id {
			return ref, nil
		}
	}
	return catalogtypes.SecretRef{}, apperr.Wrap(catalogtypes.InvalidSecretNotAvailable)
}

func (s *SecretResolverService) SecretRefs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]catalogtypes.SecretRef, error) {
	return s.secretRepository.Refs(ctx, ids)
}

func (s *SecretResolverService) ResolveSecret(ctx context.Context, id uuid.UUID) (string, error) {
	stored, err := s.secretRepository.Stored(ctx, id)
	if err != nil {
		return "", err
	}
	if stored == nil {
		return "", apperr.ErrNotFound
	}
	if stored.Ref != nil {
		return config.ResolveSecretRef(*stored.Ref)
	}
	if s.box == nil {
		return "", catalogtypes.ConflictSecretsKeyMissing
	}
	return s.box.Open(*stored.Enc, secretbox.AAD(catalogtypes.SecretTable, id.String(), catalogtypes.SecretColumnValue))
}

func (s *SecretResolverService) AdoptSecret(ctx context.Context, c catalogtypes.LegacyCredential) (catalogtypes.AdoptedSecret, error) {
	existing, err := s.secretRepository.InScope(ctx, c.Scope)
	if err != nil {
		return catalogtypes.AdoptedSecret{}, err
	}
	taken := make(map[string]bool, len(existing))
	for _, e := range existing {
		taken[strings.ToLower(e.Name)] = true
		same, err := s.sameValue(e, c.Value)
		if err != nil {
			return catalogtypes.AdoptedSecret{}, err
		}
		if same {
			return catalogtypes.AdoptedSecret{ID: e.ID, Name: e.Name}, nil
		}
	}
	id := uuid.Must(uuid.NewV7())
	stored, err := sealSecret(s.box, id, c.Value)
	if err != nil {
		return catalogtypes.AdoptedSecret{}, err
	}
	name := catalogtypes.AdoptedSecretName(c.RecordKind+"-"+c.Name, taken)
	_, err = s.secretRepository.Insert(ctx, catalogtypes.NewSecret{ID: id, NodeID: c.Scope, Name: name,
		Description: "Adopted from " + c.RecordKind + " " + c.RecordID.String(), Stored: stored})
	if err != nil {
		return catalogtypes.AdoptedSecret{}, err
	}
	return catalogtypes.AdoptedSecret{ID: id, Name: name, Created: true}, nil
}

func (s *SecretResolverService) sameValue(e catalogtypes.ScopedSecret, v catalogtypes.SecretValue) (bool, error) {
	if v.Ref != nil {
		return e.Stored.Ref != nil && *e.Stored.Ref == *v.Ref, nil
	}
	if e.Stored.Enc == nil {
		return false, nil
	}
	if s.box == nil {
		return false, catalogtypes.ConflictSecretsKeyMissing
	}
	plain, err := s.box.Open(*e.Stored.Enc, secretbox.AAD(catalogtypes.SecretTable, e.ID.String(), catalogtypes.SecretColumnValue))
	if err != nil {
		return false, err
	}
	return plain == *v.Value, nil
}

func (s *SecretResolverService) PrepareSecretRotation(ctx context.Context) (func(context.Context) error, int, error) {
	rows, err := s.secretRepository.Sealed(ctx)
	if err != nil {
		return nil, 0, err
	}
	plain := make([]string, len(rows))
	for i, r := range rows {
		v, err := s.box.Open(r.Enc, secretbox.AAD(catalogtypes.SecretTable, r.ID.String(), catalogtypes.SecretColumnValue))
		if err != nil {
			return nil, 0, err
		}
		plain[i] = v
	}
	apply := func(ctx context.Context) error {
		for i, r := range rows {
			enc, err := s.box.Seal(plain[i], secretbox.AAD(catalogtypes.SecretTable, r.ID.String(), catalogtypes.SecretColumnValue))
			if err != nil {
				return err
			}
			if err := s.secretRepository.ReplaceValue(ctx, r.ID, enc); err != nil {
				return err
			}
		}
		return nil
	}
	return apply, len(rows), nil
}

type SecretResolverService struct {
	box              *secretbox.Box
	secretRepository *repositories.SecretRepository
}
