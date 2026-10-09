package testsupport

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const DefaultTestDatabaseURL = "postgres://postgres:postgres@127.0.0.1:5434/postgres"

var counter atomic.Uint32

func TestDatabaseURL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return DefaultTestDatabaseURL
}

type TestDB struct {
	Schema string
	Pool   *pgxpool.Pool
	URL    string
}

func NewTestDB(t testing.TB) *TestDB {
	t.Helper()
	ctx := context.Background()
	adminURL := TestDatabaseURL()
	schema := fmt.Sprintf("t_%d_%d_%d", os.Getpid(), counter.Add(1), time.Now().Nanosecond())
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("test database %s unreachable (%v); run `just test-db`", adminURL, err)
	}
	if _, err := admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	_ = admin.Close(ctx)

	sep := "?"
	if strings.Contains(adminURL, "?") {
		sep = "&"
	}
	url := adminURL + sep + "options=-csearch_path%3D" + schema
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("TEST_DATABASE_URL: %v", err)
	}
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect test pool: %v", err)
	}
	db := &TestDB{Schema: schema, Pool: pool, URL: url}
	t.Cleanup(func() {
		pool.Close()
		if admin, err := pgx.Connect(context.Background(), adminURL); err == nil {
			_, _ = admin.Exec(context.Background(), `DROP SCHEMA "`+schema+`" CASCADE`)
			_ = admin.Close(context.Background())
		}
	})
	return db
}

func (d *TestDB) Tables(t testing.TB) []string {
	t.Helper()
	rows, err := d.Pool.Query(context.Background(),
		"SELECT table_name::text FROM information_schema.tables WHERE table_schema = $1 ORDER BY 1", d.Schema)
	if err != nil {
		t.Fatal(err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return names
}
