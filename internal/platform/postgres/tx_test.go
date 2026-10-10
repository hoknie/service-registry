package postgres_test

import (
	"context"
	"errors"
	"testing"

	"svc-registry/internal/platform/postgres"
	"svc-registry/internal/testsupport"
)

func count(t *testing.T, q postgres.Querier) int {
	t.Helper()
	var n int
	if err := q.QueryRow(context.Background(), "SELECT count(*) FROM items").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func newDB(t *testing.T) *postgres.DB {
	t.Helper()
	tdb := testsupport.NewTestDB(t)
	if _, err := tdb.Pool.Exec(context.Background(), "CREATE TABLE items (n int)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	return postgres.New(tdb.Pool)
}

func TestInTxCommitsAndSharesTheTransactionThroughTheContext(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	ctx := context.Background()
	err := db.InTx(ctx, func(ctx context.Context) error {
		if _, err := db.From(ctx).Exec(ctx, "INSERT INTO items VALUES (1)"); err != nil {
			return err
		}
		if got := count(t, db.From(context.Background())); got != 0 {
			t.Errorf("outside the transaction: %d rows, want 0", got)
		}
		if got := count(t, db.From(ctx)); got != 1 {
			t.Errorf("inside the transaction: %d rows, want 1", got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InTx: %v", err)
	}
	if got := count(t, db.From(ctx)); got != 1 {
		t.Fatalf("after commit: %d rows, want 1", got)
	}
}

func TestInTxRollsBackOnError(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	ctx := context.Background()
	boom := errors.New("boom")
	err := db.InTx(ctx, func(ctx context.Context) error {
		if _, err := db.From(ctx).Exec(ctx, "INSERT INTO items VALUES (1)"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("InTx error = %v, want boom", err)
	}
	if got := count(t, db.From(ctx)); got != 0 {
		t.Fatalf("after rollback: %d rows, want 0", got)
	}
}

func TestNestedInTxRollsBackOnlyItsSavepoint(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	ctx := context.Background()
	err := db.InTx(ctx, func(ctx context.Context) error {
		if _, err := db.From(ctx).Exec(ctx, "INSERT INTO items VALUES (1)"); err != nil {
			return err
		}
		inner := db.InTx(ctx, func(ctx context.Context) error {
			if _, err := db.From(ctx).Exec(ctx, "INSERT INTO items VALUES (2)"); err != nil {
				return err
			}
			return errors.New("inner")
		})
		if inner == nil {
			t.Error("inner InTx error = nil, want an error")
		}
		return db.InTx(ctx, func(ctx context.Context) error {
			_, err := db.From(ctx).Exec(ctx, "INSERT INTO items VALUES (3)")
			return err
		})
	})
	if err != nil {
		t.Fatalf("InTx: %v", err)
	}
	if got := count(t, db.From(ctx)); got != 2 {
		t.Fatalf("after commit: %d rows, want 2 (outer and the second nested)", got)
	}
}
