package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"svc-registry/internal/testsupport"
)

const trgmIndexes = "SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname IN ('nodes_name_trgm_idx', 'nodes_slug_trgm_idx')"

func TestNodeNameTrigramIndexesComeAndGo(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	eq(t, scalar[int64](t, tdb, trgmIndexes), int64(2))
	_, err := rollbackThrough(t, embedded(t), tdb, squashMigration)
	must(t, err)
	eq(t, scalar[int64](t, tdb, trgmIndexes), int64(0))
	eq(t, scalar[bool](t, tdb, "SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm')"), true, "extension stays")
}

func TestNodeNameTrigramMigrationByDatabaseRole(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		owner   bool
		indexes int64
	}{
		{"owner creates the trusted extension", true, 2},
		{"role without CREATE on the database", false, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			pool := roleDatabase(t, c.owner)
			ctx := context.Background()
			_, err := embedded(t).Migrate(ctx, pool)
			must(t, err)
			var ext bool
			must(t, pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm')").Scan(&ext))
			eq(t, ext, c.owner)
			var n int64
			must(t, pool.QueryRow(ctx, trgmIndexes).Scan(&n))
			eq(t, n, c.indexes)
		})
	}
}

func roleDatabase(t *testing.T, owner bool) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	name := fmt.Sprintf("trgm_%d", time.Now().UnixNano())
	run := func(database string, queries ...string) {
		cfg, err := pgx.ParseConfig(testsupport.TestDatabaseURL())
		must(t, err)
		if database != "" {
			cfg.Database = database
		}
		admin, err := pgx.ConnectConfig(context.Background(), cfg)
		must(t, err)
		defer func() { _ = admin.Close(context.Background()) }()
		for _, q := range queries {
			_, err := admin.Exec(context.Background(), q)
			must(t, err)
		}
	}
	create := "CREATE DATABASE " + name
	if owner {
		create += " OWNER " + name
	}
	run("", "CREATE ROLE "+name+" LOGIN PASSWORD 'trgm'", create)
	t.Cleanup(func() { run("", "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)", "DROP ROLE IF EXISTS "+name) })
	run(name, "DO $$ BEGIN IF EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'vector') THEN CREATE EXTENSION IF NOT EXISTS vector; END IF; END $$")
	if !owner {
		run(name, "GRANT CREATE ON SCHEMA public TO "+name)
	}
	cfg, err := pgx.ParseConfig(testsupport.TestDatabaseURL())
	must(t, err)
	cfg.Database, cfg.User, cfg.Password = name, name, "trgm"
	pcfg, err := pgxpool.ParseConfig("")
	must(t, err)
	pcfg.ConnConfig = cfg
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	must(t, err)
	t.Cleanup(pool.Close)
	return pool
}
