package app

import (
	"context"

	accessservices "svc-registry/internal/feature/access/services"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/secretbox"
)

func Migrate(ctx context.Context, db config.DbConfig) (int, error) {
	return postgres.Migrate(ctx, db)
}

func Rollback(ctx context.Context, db config.DbConfig, count int) (int, error) {
	return postgres.Rollback(ctx, db, count)
}

func OpenAccounts(db config.DbConfig, hash config.PasswordHashConfig) (*accessservices.AccessServices, func(), error) {
	pool, err := postgres.ConnectLazy(db)
	if err != nil {
		return nil, nil, err
	}
	return accessservices.New(accessservices.Deps{DB: postgres.New(pool), Hasher: auth.NewPasswordHasher(hash)}), pool.Close, nil
}

func OpenSecretRotation(db config.DbConfig, keys config.SecretsConfig) (func(context.Context) (int, error), func(), error) {
	pool, err := postgres.ConnectLazy(db)
	if err != nil {
		return nil, nil, err
	}
	rotate := func(ctx context.Context) (int, error) {
		return rotateSecretsOver(ctx, postgres.New(pool), secretbox.New(keys.Keys))
	}
	return rotate, pool.Close, nil
}
