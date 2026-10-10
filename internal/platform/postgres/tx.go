package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct{ pool *pgxpool.Pool }

type txKey struct{}

func New(pool *pgxpool.Pool) *DB { return &DB{pool: pool} }

func (d *DB) Pool() *pgxpool.Pool { return d.pool }

func (d *DB) From(ctx context.Context) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return d.pool
}

func (d *DB) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return d.InTxOpts(ctx, pgx.TxOptions{}, fn)
}

func (d *DB) InTxOpts(ctx context.Context, opts pgx.TxOptions, fn func(ctx context.Context) error) error {
	if outer, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return pgx.BeginFunc(ctx, outer, func(tx pgx.Tx) error { return fn(context.WithValue(ctx, txKey{}, tx)) })
	}
	return pgx.BeginTxFunc(ctx, d.pool, opts, func(tx pgx.Tx) error { return fn(context.WithValue(ctx, txKey{}, tx)) })
}

func (d *DB) Begin(ctx context.Context) (pgx.Tx, error) {
	if outer, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return outer.Begin(ctx)
	}
	return d.pool.Begin(ctx)
}
