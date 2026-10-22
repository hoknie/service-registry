package app

import (
	"context"

	catalogservices "svc-registry/internal/feature/catalog/services"
	deployservices "svc-registry/internal/feature/deploy/services"
	forgeservices "svc-registry/internal/feature/forge/services"
	forgetypes "svc-registry/internal/feature/forge/types"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
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
	return rotateSecrets(ctx, a.Secrets, a.Catalog.SecretResolver, a.Forge.Ports, a.Deploy.Ports, a.Knowledge.Ports)
}

func rotateSecretsOver(ctx context.Context, db *postgres.DB, box *secretbox.Box) (int, error) {
	cat := catalogservices.New(catalogservices.Deps{DB: db, Config: &config.Config{}, Secrets: box})
	return rotateSecrets(ctx, box, cat.SecretResolver, forgeservices.New(forgeservices.Deps{DB: db, Secrets: box}).Ports, deployservices.New(deployservices.Deps{DB: db, Secrets: box}).Ports,
		knowledgeservices.New(knowledgeservices.Deps{DB: db, Secrets: box}).Ports)
}
