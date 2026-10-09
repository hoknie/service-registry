package links

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/links"
)

type KindStore struct{ pool *pgxpool.Pool }

func NewKindStore(pool *pgxpool.Pool) *KindStore { return &KindStore{pool: pool} }

var (
	kindList   = "SELECT " + kindColumns + " FROM link_kinds k ORDER BY k.position, k.key"
	kindInsert = "INSERT INTO link_kinds AS k (id, key, names, icon, position) VALUES ($1, $2, $3, $4, $5) RETURNING " + kindColumns
	kindUpdate = "UPDATE link_kinds AS k SET names = COALESCE($2, k.names), icon = COALESCE($3, k.icon), " +
		"position = COALESCE($4, k.position), updated_at = now() WHERE k.key = $1 RETURNING " + kindColumns
	kindDelete = "DELETE FROM link_kinds WHERE key = $1"
)

func (s *KindStore) List(ctx context.Context) ([]domain.Kind, error) {
	rows, err := s.pool.Query(ctx, kindList)
	if err != nil {
		return nil, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[kindRow])
	if err != nil {
		return nil, dbErr(err)
	}
	out := make([]domain.Kind, 0, len(items))
	for _, r := range items {
		out = append(out, r.kind())
	}
	return out, nil
}

func (s *KindStore) Insert(ctx context.Context, k domain.NewKind) (domain.Kind, error) {
	rows, err := s.pool.Query(ctx, kindInsert, k.ID, k.Key, map[string]string(k.Names), string(k.Icon), k.Position)
	if err != nil {
		return domain.Kind{}, dbErr(err)
	}
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[kindRow])
	if err != nil {
		return domain.Kind{}, dbErr(err)
	}
	return r.kind(), nil
}

func (s *KindStore) Update(ctx context.Context, key string, c domain.KindChanges) (*domain.Kind, error) {
	var names any
	if c.Names != nil {
		names = map[string]string(c.Names)
	}
	var icon *string
	if c.Icon != nil {
		v := string(*c.Icon)
		icon = &v
	}
	rows, err := s.pool.Query(ctx, kindUpdate, key, names, icon, c.Position)
	if err != nil {
		return nil, dbErr(err)
	}
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[kindRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	k := r.kind()
	return &k, nil
}

func (s *KindStore) Delete(ctx context.Context, key string) error {
	tag, err := s.pool.Exec(ctx, kindDelete, key)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
