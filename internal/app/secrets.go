package app

import (
	"context"

	deployservice "svc-registry/internal/feature/deploy/service"
	"svc-registry/internal/feature/forge"
	forgeservice "svc-registry/internal/feature/forge/service"
	knowledgeservice "svc-registry/internal/feature/knowledge/service"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/secretbox"
)

type secretRotation interface {
	PrepareSecretRotation(ctx context.Context) (func(context.Context) error, int, error)
}

func rotateSecrets(ctx context.Context, box *secretbox.Box, parts ...secretRotation) (int, error) {
	if !box.CanEncrypt() {
		return 0, forge.ConflictSecretsKeyMissing
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
	return rotateSecrets(ctx, a.Secrets, a.Forge, a.Deploy, a.Knowledge)
}

func RotateSecretsOver(ctx context.Context, db *postgres.DB, box *secretbox.Box) (int, error) {
	return rotateSecrets(ctx, box, forgeservice.New(forgeservice.Deps{DB: db, Secrets: box}), deployservice.New(deployservice.Deps{DB: db, Secrets: box}),
		knowledgeservice.New(knowledgeservice.Deps{DB: db, Secrets: box}))
}
