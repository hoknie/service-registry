package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	deployservices "svc-registry/internal/feature/deploy/services"
	forgeservices "svc-registry/internal/feature/forge/services"
	forgetypes "svc-registry/internal/feature/forge/types"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	notifyservices "svc-registry/internal/feature/notify/services"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
	"svc-registry/pkg/secretbox"
)

type secretRotation interface {
	PrepareSecretRotation(ctx context.Context) (func(context.Context) error, int, error)
}

func rotateSecrets(ctx context.Context, box *secretbox.Box, parts ...secretRotation) (int, error) {
	if !box.CanEncrypt() {
		return 0, forgetypes.ConflictSecretsKeyMissing
	}
	applies := make([]func(context.Context) error, 0, len(parts))
	n := 0
	for _, p := range parts {
		apply, count, err := p.PrepareSecretRotation(ctx)
		if err != nil {
			return 0, err
		}
		applies = append(applies, apply)
		n += count
	}
	for _, apply := range applies {
		if err := apply(ctx); err != nil {
			return 0, err
		}
	}
	return n, nil
}

func (a *App) RotateSecrets(ctx context.Context) (int, error) {
	return rotateSecrets(ctx, a.Secrets, a.Catalog.SecretResolver, a.Forge.Ports, a.Notify.Ports)
}

func rotateSecretsOver(ctx context.Context, db *pgtx.DB, box *secretbox.Box) (int, error) {
	cat := catalogservices.New(catalogservices.Deps{DB: db, Config: &config.Config{}, Secrets: box})
	return rotateSecrets(ctx, box, cat.SecretResolver, forgeservices.New(forgeservices.Deps{DB: db, Secrets: box}).Ports,
		notifyservices.New(notifyservices.Deps{DB: db, Secrets: box}).Ports)
}

type legacyCredentials interface {
	AdoptLegacyCredentials(ctx context.Context, adopt catalogtypes.AdoptFunc) (int, error)
	LegacyCredentialCount(ctx context.Context) (int, error)
}

type AdoptedCredential struct {
	Kind     string
	RecordID uuid.UUID
	Secret   string
	Created  bool
}

type Adoption struct {
	Adopted     []AdoptedCredential
	Credentials int
	Secrets     int
}

type LegacyCredentialsError struct{ Records int }

func (e LegacyCredentialsError) Error() string {
	return fmt.Sprintf(`%d record(s) still use inline credentials; run "svc-registry secrets:adopt-legacy", then "db:migrate" again`, e.Records)
}

var errDryRun = errors.New("dry run")

func legacyParts(db *pgtx.DB, box *secretbox.Box) []legacyCredentials {
	return []legacyCredentials{
		forgeservices.New(forgeservices.Deps{DB: db, Secrets: box}).Ports,
		knowledgeservices.New(knowledgeservices.Deps{DB: db, Secrets: box}).Ports,
		deployservices.New(deployservices.Deps{DB: db, Secrets: box}).Ports,
	}
}

func adoptLegacy(ctx context.Context, db *pgtx.DB, box *secretbox.Box, dryRun bool) (Adoption, error) {
	resolver := catalogservices.New(catalogservices.Deps{DB: db, Config: &config.Config{}, Secrets: box}).SecretResolver
	var out Adoption
	err := db.InTx(ctx, func(ctx context.Context) error {
		used := map[uuid.UUID]bool{}
		adopt := func(ctx context.Context, c catalogtypes.LegacyCredential) (uuid.UUID, error) {
			a, err := resolver.AdoptSecret(ctx, c)
			if err != nil {
				return uuid.Nil, err
			}
			used[a.ID] = true
			out.Adopted = append(out.Adopted, AdoptedCredential{Kind: c.RecordKind, RecordID: c.RecordID, Secret: a.Name, Created: a.Created})
			return a.ID, nil
		}
		for _, p := range legacyParts(db, box) {
			n, err := p.AdoptLegacyCredentials(ctx, adopt)
			if err != nil {
				return err
			}
			out.Credentials += n
		}
		out.Secrets = len(used)
		if dryRun {
			return errDryRun
		}
		return nil
	})
	if errors.Is(err, errDryRun) {
		err = nil
	}
	if err != nil {
		return Adoption{}, err
	}
	return out, nil
}

func legacyCount(ctx context.Context, db *pgtx.DB) error {
	n := 0
	for _, p := range legacyParts(db, secretbox.New(nil)) {
		c, err := p.LegacyCredentialCount(ctx)
		if err != nil {
			return err
		}
		n += c
	}
	if n > 0 {
		return LegacyCredentialsError{Records: n}
	}
	return nil
}
