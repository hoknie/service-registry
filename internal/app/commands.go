package app

import (
	"context"

	accessservices "svc-registry/internal/feature/access/services"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/password"
	"svc-registry/pkg/pgtx"
	"svc-registry/pkg/secretbox"
)

const legacyCredentialsBarrier = 20261022100002

func Migrate(ctx context.Context, db config.DbConfig) (int, error) {
	pool, err := postgres.ConnectLazy(db)
	if err != nil {
		return 0, err
	}
	defer pool.Close()
	tx := pgtx.New(pool)
	return postgres.Embedded().MigrateUntil(ctx, pool, legacyCredentialsBarrier, func(ctx context.Context) error {
		return legacyCount(ctx, tx)
	})
}

func Rollback(ctx context.Context, db config.DbConfig, count int) (int, error) {
	return postgres.Rollback(ctx, db, count)
}

func OpenAccounts(db config.DbConfig, hash config.PasswordHashConfig) (*accessservices.AccessServices, func(), error) {
	pool, err := postgres.ConnectLazy(db)
	if err != nil {
		return nil, nil, err
	}
	return accessservices.New(accessservices.Deps{DB: pgtx.New(pool), Hasher: password.NewHasher(hash.Params())}), pool.Close, nil
}

func OpenSecretRotation(db config.DbConfig, keys config.SecretsConfig) (func(context.Context) (int, error), func(), error) {
	pool, err := postgres.ConnectLazy(db)
	if err != nil {
		return nil, nil, err
	}
	rotate := func(ctx context.Context) (int, error) {
		return rotateSecretsOver(ctx, pgtx.New(pool), secretbox.New(keys.Keys))
	}
	return rotate, pool.Close, nil
}

func OpenLegacyAdoption(db config.DbConfig, keys config.SecretsConfig) (func(context.Context, bool) (Adoption, error), func(), error) {
	pool, err := postgres.ConnectLazy(db)
	if err != nil {
		return nil, nil, err
	}
	adopt := func(ctx context.Context, dryRun bool) (Adoption, error) {
		return adoptLegacy(ctx, pgtx.New(pool), secretbox.New(keys.Keys), dryRun)
	}
	return adopt, pool.Close, nil
}
