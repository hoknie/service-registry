package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

type KindRepository struct{ db *postgres.DB }

func NewKindRepository(db *postgres.DB) *KindRepository { return &KindRepository{db: db} }

func (s *KindRepository) List(ctx context.Context) ([]linkstypes.Kind, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT k.id, k.key, k.names, k.icon, k.icon_url, k.position, rfc3339(k.created_at) AS created_at,
			rfc3339(k.updated_at) AS updated_at
		FROM link_kinds k
		ORDER BY k.position, k.key`)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[kindRow])
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out := make([]linkstypes.Kind, 0, len(items))
	for _, r := range items {
		out = append(out, r.kind())
	}
	return out, nil
}

func (s *KindRepository) Insert(ctx context.Context, k linkstypes.NewKind) (linkstypes.Kind, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		INSERT INTO link_kinds AS k (id, key, names, icon, icon_url, position)
		SELECT $1, $2, $3, $4, $5, COALESCE($6, LEAST(COALESCE(max(position), 0) + 10, 10000))
		FROM link_kinds
		RETURNING k.id, k.key, k.names, k.icon, k.icon_url, k.position, rfc3339(k.created_at) AS created_at,
			rfc3339(k.updated_at) AS updated_at`, k.ID, k.Key, map[string]string(k.Names), k.Icon, k.IconURL, k.Position)
	if err != nil {
		return linkstypes.Kind{}, postgres.Translate(err, constraints)
	}
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[kindRow])
	if err != nil {
		return linkstypes.Kind{}, postgres.Translate(err, constraints)
	}
	return r.kind(), nil
}

func (s *KindRepository) Update(ctx context.Context, key string, c linkstypes.KindChanges) (*linkstypes.Kind, error) {
	var names any
	if c.Names != nil {
		names = map[string]string(c.Names)
	}
	rows, err := s.db.From(ctx).Query(ctx, `
		UPDATE link_kinds AS k
		SET names = COALESCE($2, k.names), icon = COALESCE($3, k.icon),
			icon_url = CASE WHEN $4 THEN $5 ELSE k.icon_url END, position = COALESCE($6, k.position),
			updated_at = now()
		WHERE k.key = $1
		RETURNING k.id, k.key, k.names, k.icon, k.icon_url, k.position, rfc3339(k.created_at) AS created_at,
			rfc3339(k.updated_at) AS updated_at`, key, names, c.Icon, c.IconURLSet, c.IconURL, c.Position)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[kindRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	k := r.kind()
	return &k, nil
}

func (s *KindRepository) Reorder(ctx context.Context, keys []string) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		rows, err := tx.Query(ctx, `
			SELECT key
			FROM link_kinds
			ORDER BY key
			FOR UPDATE`)
		if err != nil {
			return err
		}
		current, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		if len(current) != len(keys) {
			return linkstypes.ConflictKindOrderStale
		}
		given := make(map[string]bool, len(keys))
		for _, k := range keys {
			given[k] = true
		}
		for _, k := range current {
			if !given[k] {
				return linkstypes.ConflictKindOrderStale
			}
		}
		_, err = tx.Exec(ctx, `
			UPDATE link_kinds AS k
			SET position = o.ord * 10, updated_at = now()
			FROM unnest($1::text[]) WITH ORDINALITY AS o(key, ord)
			WHERE k.key = o.key
				AND k.position <> o.ord * 10`, keys)
		return err
	})
	return postgres.Translate(err, constraints)
}

func (s *KindRepository) Delete(ctx context.Context, key string) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM link_kinds
		WHERE key = $1`, key)
	if err != nil {
		return postgres.Translate(err, constraints)
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}
	return nil
}
