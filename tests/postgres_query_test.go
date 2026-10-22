package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/internal/testsupport"
	"svc-registry/pkg/pgtx"
)

var errItemTaken = apperr.Conflicting("conflict.item_taken", "item exists")

type itemRow struct {
	N int64 `db:"n"`
}

func (r itemRow) value() int64 { return r.N * 10 }

func queryDB(t *testing.T) pgtx.Querier {
	t.Helper()
	tdb := testsupport.NewTestDB(t)
	if _, err := tdb.Pool.Exec(context.Background(), "CREATE TABLE items (n bigint CONSTRAINT items_n_key UNIQUE)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := tdb.Pool.Exec(context.Background(), "INSERT INTO items VALUES (1), (2)"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return pgtx.New(tdb.Pool).From(context.Background())
}

func TestQueryFunctionsCollectAndTranslate(t *testing.T) {
	t.Parallel()
	q := queryDB(t)
	ctx := context.Background()
	c := postgres.Constraints{"items_n_key": errItemTaken}
	all, err := postgres.List(ctx, q, c, postgres.Struct(itemRow.value), "SELECT n FROM items ORDER BY n")
	if err != nil || len(all) != 2 || all[0] != 10 || all[1] != 20 {
		t.Fatalf("list: %v %v", all, err)
	}
	none, err := postgres.List(ctx, q, c, pgx.RowTo[int64], "SELECT n FROM items WHERE n > 5")
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("empty list: %#v %v", none, err)
	}
	count, err := postgres.One(ctx, q, c, pgx.RowTo[int64], "SELECT count(*) FROM items")
	if err != nil || count != 2 {
		t.Fatalf("one: %d %v", count, err)
	}
	if _, err := postgres.One(ctx, q, c, pgx.RowTo[int64], "SELECT n FROM items WHERE n > 5"); err == nil || errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("one without a row must be an internal error: %v", err)
	}
	found, err := postgres.Find(ctx, q, c, postgres.Struct(itemRow.value), "SELECT n FROM items WHERE n = $1", 2)
	if err != nil || found == nil || *found != 20 {
		t.Fatalf("find: %v %v", found, err)
	}
	missing, err := postgres.Find(ctx, q, c, postgres.Struct(itemRow.value), "SELECT n FROM items WHERE n = $1", 9)
	if err != nil || missing != nil {
		t.Fatalf("find missing: %v %v", missing, err)
	}
	if _, err := postgres.One(ctx, q, c, pgx.RowTo[int64], "INSERT INTO items VALUES (1) RETURNING n"); !errors.Is(err, errItemTaken) {
		t.Fatalf("constraint: %v", err)
	}
	if n, err := postgres.Exec(ctx, q, c, "UPDATE items SET n = n + 10"); err != nil || n != 2 {
		t.Fatalf("exec: %d %v", n, err)
	}
	if err := postgres.ExecFound(ctx, q, c, "DELETE FROM items WHERE n = $1", 99); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("exec found: %v", err)
	}
	if err := postgres.ExecFound(ctx, q, c, "DELETE FROM items WHERE n = $1", 11); err != nil {
		t.Fatalf("exec found a row: %v", err)
	}
}
