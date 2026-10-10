package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/links"
	"svc-registry/internal/platform/postgres"
)

type Kinds struct{ db *postgres.DB }

func NewKinds(db *postgres.DB) *Kinds { return &Kinds{db: db} }

func (s *Kinds) List(ctx context.Context) ([]links.Kind, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT k.id, k.key, k.names, k.icon, k.position, rfc3339(k.created_at) AS created_at,
			rfc3339(k.updated_at) AS updated_at
		FROM link_kinds k
		ORDER BY k.position, k.key`)
	if err != nil {
		return nil, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[kindRow])
	if err != nil {
		return nil, dbErr(err)
	}
	out := make([]links.Kind, 0, len(items))
	for _, r := range items {
		out = append(out, r.kind())
	}
	return out, nil
}

func (s *Kinds) Insert(ctx context.Context, k links.NewKind) (links.Kind, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		INSERT INTO link_kinds AS k (id, key, names, icon, position)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING k.id, k.key, k.names, k.icon, k.position, rfc3339(k.created_at) AS created_at,
			rfc3339(k.updated_at) AS updated_at`, k.ID, k.Key, map[string]string(k.Names), string(k.Icon), k.Position)
	if err != nil {
		return links.Kind{}, dbErr(err)
	}
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[kindRow])
	if err != nil {
		return links.Kind{}, dbErr(err)
	}
	return r.kind(), nil
}

func (s *Kinds) Update(ctx context.Context, key string, c links.KindChanges) (*links.Kind, error) {
	var names any
	if c.Names != nil {
		names = map[string]string(c.Names)
	}
	var icon *string
	if c.Icon != nil {
		v := string(*c.Icon)
		icon = &v
	}
	rows, err := s.db.From(ctx).Query(ctx, `
		UPDATE link_kinds AS k
		SET names = COALESCE($2, k.names), icon = COALESCE($3, k.icon), position = COALESCE($4, k.position),
			updated_at = now()
		WHERE k.key = $1
		RETURNING k.id, k.key, k.names, k.icon, k.position, rfc3339(k.created_at) AS created_at,
			rfc3339(k.updated_at) AS updated_at`, key, names, icon, c.Position)
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

func (s *Kinds) Delete(ctx context.Context, key string) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM link_kinds
		WHERE key = $1`, key)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return links.ErrNotFound
	}
	return nil
}
