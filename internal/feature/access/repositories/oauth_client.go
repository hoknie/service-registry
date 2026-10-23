package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type OAuthClientRepository struct{ db *pgtx.DB }

func NewOAuthClientRepository(db *pgtx.DB) *OAuthClientRepository {
	return &OAuthClientRepository{db: db}
}

func scanOAuthClient(row pgx.CollectableRow) (accesstypes.OAuthClientRecord, error) {
	var c accesstypes.OAuthClientRecord
	err := row.Scan(&c.ID, &c.Name, &c.RedirectURIs, &c.MetadataURL, &c.Fresh, &c.CreatedAt, &c.CreatedUnix)
	return c, err
}

func (s *OAuthClientRepository) Insert(ctx context.Context, id uuid.UUID, meta accesstypes.ClientMetadata) (accesstypes.OAuthClientRecord, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, scanOAuthClient, `
		INSERT INTO oauth_clients AS c (id, name, redirect_uris)
		VALUES ($1, $2, $3)
		RETURNING c.id, c.name, c.redirect_uris, c.metadata_url, true AS fresh, rfc3339(c.created_at) AS created_at,
			extract(epoch FROM c.created_at)::bigint AS created_unix`, id, meta.Name, meta.RedirectURIs)
}

func (s *OAuthClientRepository) Find(ctx context.Context, id uuid.UUID) (*accesstypes.OAuthClientRecord, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanOAuthClient, `
		UPDATE oauth_clients AS c
		SET last_used_at = now()
		WHERE c.id = $1 AND c.metadata_url IS NULL
		RETURNING c.id, c.name, c.redirect_uris, c.metadata_url, true AS fresh, rfc3339(c.created_at) AS created_at,
			extract(epoch FROM c.created_at)::bigint AS created_unix`, id)
}

func (s *OAuthClientRepository) FindDocument(ctx context.Context, metadataURL string) (*accesstypes.OAuthClientRecord, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanOAuthClient, `
		UPDATE oauth_clients AS c
		SET last_used_at = now()
		WHERE c.metadata_url = $1
		RETURNING c.id, c.name, c.redirect_uris, c.metadata_url, c.metadata_expires_at > now() AS fresh,
			rfc3339(c.created_at) AS created_at, extract(epoch FROM c.created_at)::bigint AS created_unix`, metadataURL)
}

func (s *OAuthClientRepository) SaveDocument(ctx context.Context, id uuid.UUID, metadataURL string, meta accesstypes.ClientMetadata, lifeSecs int64) (accesstypes.OAuthClientRecord, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, scanOAuthClient, `
		INSERT INTO oauth_clients AS c (id, name, redirect_uris, metadata_url, metadata_expires_at, last_used_at)
		VALUES ($1, $2, $3, $4, now() + make_interval(secs => $5), now())
		ON CONFLICT (metadata_url) DO UPDATE
		SET name = excluded.name, redirect_uris = excluded.redirect_uris,
			metadata_expires_at = excluded.metadata_expires_at, last_used_at = now()
		RETURNING c.id, c.name, c.redirect_uris, c.metadata_url, true AS fresh, rfc3339(c.created_at) AS created_at,
			extract(epoch FROM c.created_at)::bigint AS created_unix`, id, meta.Name, meta.RedirectURIs, metadataURL, lifeSecs)
}

func (s *OAuthClientRepository) DocumentClient(ctx context.Context, metadataURL string) (*uuid.UUID, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		SELECT id FROM oauth_clients WHERE metadata_url = $1`, metadataURL)
}

func (s *OAuthClientRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT EXISTS (SELECT 1 FROM oauth_clients WHERE id = $1 AND metadata_url IS NULL)`, id)
}

func (s *OAuthClientRepository) PruneAbandoned(ctx context.Context) (int64, error) {
	return postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM oauth_clients c
		WHERE c.created_at < now() - interval '30 days'
			AND NOT EXISTS (SELECT 1 FROM oauth_grants g WHERE g.client_id = c.id)`)
}
