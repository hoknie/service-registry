package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"svc-registry/internal/testsupport"
	"svc-registry/migrations"
)

const trgmIndexes = "SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname IN ('nodes_name_trgm_idx', 'nodes_slug_trgm_idx')"

func TestNodeNameTrigramIndexesComeAndGo(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	eq(t, scalar[int64](t, tdb, trgmIndexes), int64(2))
	_, err := rollbackThrough(t, embedded(t), tdb, "alter_nodes_name_trgm")
	must(t, err)
	eq(t, scalar[int64](t, tdb, trgmIndexes), int64(0))
	eq(t, scalar[bool](t, tdb, "SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm')"), true, "extension stays")
}

func TestNodeNameTrigramMigrationByDatabaseRole(t *testing.T) {
	t.Parallel()
	up, err := migrations.FS.ReadFile("20261022100001_alter_nodes_name_trgm.up.sql")
	must(t, err)
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
			conn := roleDatabase(t, c.owner)
			ctx := context.Background()
			_, err := conn.Exec(ctx, "CREATE TABLE nodes (id uuid PRIMARY KEY, name text NOT NULL, slug text NOT NULL)")
			must(t, err)
			_, err = conn.Exec(ctx, string(up))
			must(t, err)
			var ext bool
			must(t, conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm')").Scan(&ext))
			eq(t, ext, c.owner)
			var n int64
			must(t, conn.QueryRow(ctx, trgmIndexes).Scan(&n))
			eq(t, n, c.indexes)
		})
	}
}

func roleDatabase(t *testing.T, owner bool) *pgx.Conn {
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
	if !owner {
		run(name, "GRANT CREATE ON SCHEMA public TO "+name)
	}
	cfg, err := pgx.ParseConfig(testsupport.TestDatabaseURL())
	must(t, err)
	cfg.Database, cfg.User, cfg.Password = name, name, "trgm"
	conn, err := pgx.ConnectConfig(ctx, cfg)
	must(t, err)
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}
