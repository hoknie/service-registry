package repositories

import (
	"context"

	"github.com/jackc/pgx/v5"

	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type KindRepository struct{ db *pgtx.DB }

func NewKindRepository(db *pgtx.DB) *KindRepository { return &KindRepository{db: db} }

func (s *KindRepository) List(ctx context.Context) ([]linkstypes.Kind, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(kindRow.kind), `
		SELECT k.id, k.key, k.names, k.icon, k.icon_url, k.position, rfc3339(k.created_at) AS created_at,
			rfc3339(k.updated_at) AS updated_at
		FROM link_kinds k
		ORDER BY k.position, k.key`)
}

func (s *KindRepository) Insert(ctx context.Context, k linkstypes.NewKind) (linkstypes.Kind, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, postgres.Struct(kindRow.kind), `
		INSERT INTO link_kinds AS k (id, key, names, icon, icon_url, position)
		SELECT $1, $2, $3, $4, $5, COALESCE($6, LEAST(COALESCE(max(position), 0) + 10, 10000))
		FROM link_kinds
		RETURNING k.id, k.key, k.names, k.icon, k.icon_url, k.position, rfc3339(k.created_at) AS created_at,
			rfc3339(k.updated_at) AS updated_at`, k.ID, k.Key, map[string]string(k.Names), k.Icon, k.IconURL, k.Position)
}

func (s *KindRepository) Update(ctx context.Context, key string, c linkstypes.KindChanges) (*linkstypes.Kind, error) {
	var names any
	if c.Names != nil {
		names = map[string]string(c.Names)
	}
	return postgres.Find(ctx, s.db.From(ctx), constraints, postgres.Struct(kindRow.kind), `
		UPDATE link_kinds AS k
		SET names = COALESCE($2, k.names), icon = COALESCE($3, k.icon),
			icon_url = CASE WHEN $4 THEN $5 ELSE k.icon_url END, position = COALESCE($6, k.position),
			updated_at = now()
		WHERE k.key = $1
		RETURNING k.id, k.key, k.names, k.icon, k.icon_url, k.position, rfc3339(k.created_at) AS created_at,
			rfc3339(k.updated_at) AS updated_at`, key, names, c.Icon, c.IconURLSet, c.IconURL, c.Position)
}

func (s *KindRepository) LockKeys(ctx context.Context) ([]string, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[string], `
		SELECT key
		FROM link_kinds
		ORDER BY key
		FOR UPDATE`)
}

func (s *KindRepository) SetOrder(ctx context.Context, keys []string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE link_kinds AS k
		SET position = o.ord * 10, updated_at = now()
		FROM unnest($1::text[]) WITH ORDINALITY AS o(key, ord)
		WHERE k.key = o.key
			AND k.position <> o.ord * 10`, keys)
	return err
}

func (s *KindRepository) Delete(ctx context.Context, key string) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM link_kinds
		WHERE key = $1`, key)
}
