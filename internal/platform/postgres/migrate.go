package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"svc-registry/internal/platform/config"
	"svc-registry/migrations"
)

const Ledger = "schema_migrations"

type Migrator struct{ files fs.FS }

func NewMigrator(files fs.FS) Migrator { return Migrator{files: files} }

func Embedded() Migrator { return NewMigrator(migrations.FS) }

func (m Migrator) Versions() ([]uint, error) {
	src, err := iofs.New(m.files, ".")
	if err != nil {
		return nil, err
	}
	defer src.Close()
	var out []uint
	v, err := src.First()
	for err == nil {
		out = append(out, v)
		v, err = src.Next(v)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return out, nil
}

func (m Migrator) open(pool *pgxpool.Pool) (*migrate.Migrate, error) {
	src, err := iofs.New(m.files, ".")
	if err != nil {
		return nil, err
	}
	driver, err := pgxmigrate.WithInstance(stdlib.OpenDBFromPool(pool), &pgxmigrate.Config{})
	if err != nil {
		_ = src.Close()
		return nil, err
	}
	return migrate.NewWithInstance("iofs", src, "pgx5", driver)
}

func (m Migrator) applied(mg *migrate.Migrate) (int, error) {
	version, dirty, err := mg.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if dirty {
		return 0, fmt.Errorf("migration %d failed earlier and left the schema dirty; repair it, then fix the ledger (%s)", version, Ledger)
	}
	versions, err := m.Versions()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, v := range versions {
		if v <= version {
			n++
		}
	}
	return n, nil
}

func (m Migrator) Migrate(_ context.Context, pool *pgxpool.Pool) (int, error) {
	mg, err := m.open(pool)
	if err != nil {
		return 0, err
	}
	defer mg.Close()
	before, err := m.applied(mg)
	if err != nil {
		return 0, err
	}
	if err := mg.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return 0, short(err)
	}
	after, err := m.applied(mg)
	return after - before, err
}

func (m Migrator) Rollback(_ context.Context, pool *pgxpool.Pool, count int) (int, error) {
	mg, err := m.open(pool)
	if err != nil {
		return 0, err
	}
	defer mg.Close()
	applied, err := m.applied(mg)
	if err != nil {
		return 0, err
	}
	n := min(count, applied)
	if n == 0 {
		return 0, nil
	}
	if err := mg.Steps(-n); err != nil {
		return 0, short(err)
	}
	return n, nil
}

func short(err error) error {
	var dbErr database.Error
	if errors.As(err, &dbErr) && dbErr.OrigErr != nil {
		return fmt.Errorf("line %d: %w", dbErr.Line, dbErr.OrigErr)
	}
	return err
}

func Migrate(ctx context.Context, cfg config.DbConfig) (int, error) {
	pool, err := ConnectLazy(cfg)
	if err != nil {
		return 0, err
	}
	defer pool.Close()
	return Embedded().Migrate(ctx, pool)
}

func Rollback(ctx context.Context, cfg config.DbConfig, count int) (int, error) {
	pool, err := ConnectLazy(cfg)
	if err != nil {
		return 0, err
	}
	defer pool.Close()
	return Embedded().Rollback(ctx, pool, count)
}
