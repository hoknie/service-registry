package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type RepoRepository struct{ db *pgtx.DB }

func NewRepoRepository(db *pgtx.DB) *RepoRepository { return &RepoRepository{db: db} }

func (s *RepoRepository) Links(ctx context.Context, connID uuid.UUID) ([]reposynctypes.Link, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (reposynctypes.Link, error) {
		var l reposynctypes.Link
		err := r.Scan(&l.ProjectID, &l.ExternalID, &l.FullPath, &l.Orphaned, &l.SourceUpdatedAt)
		return l, err
	}, `
		SELECT project_id, external_id, full_path, orphaned_at IS NOT NULL, source_updated_at
		FROM repo_sync_repositories
		WHERE connection_id = $1`, connID)
}

func (s *RepoRepository) Groups(ctx context.Context, connID uuid.UUID) (map[string]uuid.UUID, error) {
	return postgres.Pairs[string, uuid.UUID](ctx, s.db.From(ctx), constraints, `
		SELECT lower(full_path), node_id
		FROM repo_sync_groups
		WHERE connection_id = $1`, connID)
}

func (s *RepoRepository) LinkGroup(ctx context.Context, connID, nodeID uuid.UUID, fullPath string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO repo_sync_groups (node_id, connection_id, full_path)
		VALUES ($1, $2, $3)
		ON CONFLICT (node_id)
		DO UPDATE SET full_path = EXCLUDED.full_path`, nodeID, connID, fullPath)
	return err
}

func (s *RepoRepository) Upsert(ctx context.Context, rec reposynctypes.StoredRepo) error {
	r := rec.Remote
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO repo_sync_repositories AS f (project_id, connection_id, external_id, full_path, web_url,
			description, topics, default_branch, archived, visibility, stars, license, pushed_at,
			source_updated_at, readme_md, readme_truncated, languages)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $16, COALESCE($17, false),
			COALESCE($18, '{}'::jsonb))
		ON CONFLICT (project_id)
		DO UPDATE SET connection_id = EXCLUDED.connection_id, external_id = EXCLUDED.external_id,
			full_path = EXCLUDED.full_path, web_url = EXCLUDED.web_url, description = EXCLUDED.description,
			topics = EXCLUDED.topics, default_branch = EXCLUDED.default_branch, archived = EXCLUDED.archived,
			visibility = EXCLUDED.visibility, stars = EXCLUDED.stars, pushed_at = EXCLUDED.pushed_at,
			license = CASE WHEN $15 THEN EXCLUDED.license ELSE COALESCE(EXCLUDED.license, f.license) END,
			source_updated_at = CASE WHEN $15 THEN EXCLUDED.source_updated_at ELSE f.source_updated_at END,
			readme_md = CASE WHEN $15 THEN EXCLUDED.readme_md ELSE f.readme_md END,
			readme_truncated = CASE WHEN $15 THEN EXCLUDED.readme_truncated ELSE f.readme_truncated END,
			languages = CASE WHEN $15 THEN EXCLUDED.languages ELSE f.languages END, orphaned_at = NULL,
			synced_at = now(), updated_at = now()`, rec.ProjectID, rec.ConnectionID, r.ExternalID, r.FullPath, r.WebURL,
		r.Description, rec.Topics, rec.DefaultBranch, r.Archived, string(r.Visibility), r.Stars, rec.License, r.PushedAt,
		rec.ChangedAt, rec.WithDetails, rec.Readme, rec.Truncated, rec.Languages)
	return err
}

func (s *RepoRepository) Orphan(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE repo_sync_repositories
		SET orphaned_at = now(), updated_at = now()
		WHERE project_id = ANY($1)
			AND orphaned_at IS NULL`, ids)
	return err
}

func (s *RepoRepository) Find(ctx context.Context, projectID uuid.UUID) (*reposynctypes.Repository, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(row pgx.CollectableRow) (reposynctypes.Repository, error) {
		var r reposynctypes.Repository
		var vis, kind string
		err := row.Scan(&r.ConnectionID, &r.ExternalID, &r.FullPath, &r.WebURL, &r.Description, &r.Topics, &r.Languages,
			&r.DefaultBranch, &r.Archived, &vis, &r.Stars, &r.License, &r.PushedAt, &r.SyncedAt, &r.OrphanedAt,
			&r.HasReadme, &kind)
		r.Visibility = reposynctypes.Visibility(vis)
		r.Kind = reposynctypes.Kind(kind)
		return r, err
	}, `
		SELECT f.connection_id, f.external_id, f.full_path, f.web_url, f.description, f.topics, f.languages,
			f.default_branch, f.archived, f.visibility, f.stars, f.license, rfc3339(f.pushed_at),
			rfc3339(f.synced_at), rfc3339(f.orphaned_at), f.readme_md IS NOT NULL, c.kind
		FROM repo_sync_repositories f
		JOIN repo_sync_connections c ON c.id = f.connection_id
		WHERE f.project_id = $1`, projectID)
}

func (s *RepoRepository) Managed(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]bool{}, nil
	}
	return postgres.Set[uuid.UUID](ctx, s.db.From(ctx), constraints, `
		SELECT project_id
		FROM repo_sync_repositories
		WHERE project_id = ANY($1)
			AND orphaned_at IS NULL
		UNION
		SELECT node_id
		FROM repo_sync_groups
		WHERE node_id = ANY($1)`, ids)
}

func (s *RepoRepository) Readme(ctx context.Context, projectID uuid.UUID) (*reposynctypes.Readme, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(row pgx.CollectableRow) (reposynctypes.Readme, error) {
		var r reposynctypes.Readme
		var kind string
		err := row.Scan(&r.Markdown, &r.Truncated, &r.WebURL, &r.DefaultBranch, &kind)
		r.Kind = reposynctypes.Kind(kind)
		return r, err
	}, `
		SELECT f.readme_md, f.readme_truncated, f.web_url, f.default_branch, c.kind
		FROM repo_sync_repositories f
		JOIN repo_sync_connections c ON c.id = f.connection_id
		WHERE f.project_id = $1
			AND f.readme_md IS NOT NULL`, projectID)
}

func (s *RepoRepository) Unlink(ctx context.Context, projectID uuid.UUID) (*reposynctypes.Unlinked, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(row pgx.CollectableRow) (reposynctypes.Unlinked, error) {
		u := reposynctypes.Unlinked{ProjectID: projectID}
		err := row.Scan(&u.ConnectionID, &u.ExternalID, &u.FullPath)
		return u, err
	}, `
		DELETE FROM repo_sync_repositories
		WHERE project_id = $1
			AND orphaned_at IS NULL
		RETURNING connection_id, external_id, full_path`, projectID)
}

func (s *RepoRepository) Link(ctx context.Context, projectID uuid.UUID) (*reposynctypes.ProjectLink, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanProjectLink, `
		SELECT f.connection_id, c.node_id, c.owner_path, c.kind, f.web_url, c.api_url, c.credentials_secret_id,
			rfc3339(f.synced_at)
		FROM repo_sync_repositories f
		JOIN repo_sync_connections c ON c.id = f.connection_id
		WHERE f.project_id = $1
			AND f.orphaned_at IS NULL`, projectID)
}
