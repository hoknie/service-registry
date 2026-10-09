package forge

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/forge"
	"svc-registry/internal/postgres"
)

type RepositoryStore struct{ pool *pgxpool.Pool }

func NewRepositoryStore(pool *pgxpool.Pool) *RepositoryStore { return &RepositoryStore{pool: pool} }

var (
	repoLinks = "SELECT project_id, external_id, full_path, orphaned_at IS NOT NULL, source_updated_at " +
		"FROM forge_repositories WHERE connection_id = $1"
	repoGroups    = "SELECT lower(full_path), node_id FROM forge_groups WHERE connection_id = $1"
	repoLinkGroup = "INSERT INTO forge_groups (node_id, connection_id, full_path) VALUES ($1, $2, $3) " +
		"ON CONFLICT (node_id) DO UPDATE SET full_path = EXCLUDED.full_path"
	repoUpsert = "INSERT INTO forge_repositories AS f (project_id, connection_id, external_id, full_path, web_url, " +
		"description, topics, default_branch, archived, visibility, stars, license, pushed_at, source_updated_at, " +
		"readme_md, readme_truncated, languages) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, " +
		"$16, COALESCE($17, false), COALESCE($18, '{}'::jsonb)) " +
		"ON CONFLICT (project_id) DO UPDATE SET connection_id = EXCLUDED.connection_id, external_id = EXCLUDED.external_id, " +
		"full_path = EXCLUDED.full_path, web_url = EXCLUDED.web_url, description = EXCLUDED.description, " +
		"topics = EXCLUDED.topics, default_branch = EXCLUDED.default_branch, archived = EXCLUDED.archived, " +
		"visibility = EXCLUDED.visibility, stars = EXCLUDED.stars, pushed_at = EXCLUDED.pushed_at, " +
		"license = CASE WHEN $15 THEN EXCLUDED.license ELSE COALESCE(EXCLUDED.license, f.license) END, " +
		"source_updated_at = CASE WHEN $15 THEN EXCLUDED.source_updated_at ELSE f.source_updated_at END, " +
		"readme_md = CASE WHEN $15 THEN EXCLUDED.readme_md ELSE f.readme_md END, " +
		"readme_truncated = CASE WHEN $15 THEN EXCLUDED.readme_truncated ELSE f.readme_truncated END, " +
		"languages = CASE WHEN $15 THEN EXCLUDED.languages ELSE f.languages END, " +
		"orphaned_at = NULL, synced_at = now(), updated_at = now()"
	repoOrphan = "UPDATE forge_repositories SET orphaned_at = now(), updated_at = now() " +
		"WHERE project_id = ANY($1) AND orphaned_at IS NULL"
	repoFind = "SELECT f.connection_id, f.external_id, f.full_path, f.web_url, f.description, f.topics, f.languages, f.default_branch, " +
		"f.archived, f.visibility, f.stars, f.license, " + postgres.RFC3339("f.pushed_at") + ", " +
		postgres.RFC3339("f.synced_at") + ", " + postgres.RFC3339("f.orphaned_at") + ", f.readme_md IS NOT NULL, c.kind " +
		"FROM forge_repositories f JOIN forge_connections c ON c.id = f.connection_id WHERE f.project_id = $1"
	repoManaged = "SELECT project_id FROM forge_repositories WHERE project_id = ANY($1) AND orphaned_at IS NULL " +
		"UNION SELECT node_id FROM forge_groups WHERE node_id = ANY($1)"
	repoReadme = "SELECT f.readme_md, f.readme_truncated, f.web_url, f.default_branch, c.kind FROM forge_repositories f " +
		"JOIN forge_connections c ON c.id = f.connection_id WHERE f.project_id = $1 AND f.readme_md IS NOT NULL"
)

func (s *RepositoryStore) Links(ctx context.Context, connID uuid.UUID) ([]domain.Link, error) {
	rows, err := s.pool.Query(ctx, repoLinks, connID)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Link, error) {
		var l domain.Link
		err := r.Scan(&l.ProjectID, &l.ExternalID, &l.FullPath, &l.Orphaned, &l.SourceUpdatedAt)
		return l, err
	})
	return out, dbErr(err)
}

func (s *RepositoryStore) Groups(ctx context.Context, connID uuid.UUID) (map[string]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, repoGroups, connID)
	if err != nil {
		return nil, dbErr(err)
	}
	out := map[string]uuid.UUID{}
	var path string
	var id uuid.UUID
	_, err = pgx.ForEachRow(rows, []any{&path, &id}, func() error {
		out[path] = id
		return nil
	})
	return out, dbErr(err)
}

func (s *RepositoryStore) LinkGroup(ctx context.Context, connID, nodeID uuid.UUID, fullPath string) error {
	_, err := s.pool.Exec(ctx, repoLinkGroup, nodeID, connID, fullPath)
	return dbErr(err)
}

func (s *RepositoryStore) Upsert(ctx context.Context, rec domain.RepoRecord) error {
	r := rec.Remote
	var branch *string
	if r.DefaultBranch != "" {
		branch = &r.DefaultBranch
	}
	var license *string
	if r.License != "" {
		license = &r.License
	}
	topics := r.Topics
	if topics == nil {
		topics = []string{}
	}
	withDetails := rec.Details != nil
	var readme *string
	var truncated *bool
	var languages map[string]float64
	var changed *time.Time
	if withDetails {
		readme, truncated, languages = rec.Details.Readme, &rec.Details.Truncated, rec.Details.Languages
		if rec.Details.License != nil {
			license = rec.Details.License
		}
		changed = r.ChangedAt()
	}
	if languages == nil {
		languages = map[string]float64{}
	}
	_, err := s.pool.Exec(ctx, repoUpsert, rec.ProjectID, rec.ConnectionID, r.ExternalID, r.FullPath, r.WebURL,
		r.Description, topics, branch, r.Archived, string(r.Visibility), r.Stars, license, r.PushedAt, changed,
		withDetails, readme, truncated, languages)
	return dbErr(err)
}

func (s *RepositoryStore) Orphan(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, repoOrphan, ids)
	return dbErr(err)
}

func (s *RepositoryStore) Find(ctx context.Context, projectID uuid.UUID) (*domain.Repository, error) {
	var r domain.Repository
	var vis, kind string
	err := s.pool.QueryRow(ctx, repoFind, projectID).Scan(&r.ConnectionID, &r.ExternalID, &r.FullPath, &r.WebURL, &r.Description,
		&r.Topics, &r.Languages, &r.DefaultBranch, &r.Archived, &vis, &r.Stars, &r.License, &r.PushedAt, &r.SyncedAt,
		&r.OrphanedAt, &r.HasReadme, &kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	r.Visibility = domain.Visibility(vis)
	r.Kind = domain.Kind(kind)
	if r.Topics == nil {
		r.Topics = []string{}
	}
	if r.Languages == nil {
		r.Languages = map[string]float64{}
	}
	return &r, nil
}

func (s *RepositoryStore) Managed(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, repoManaged, ids)
	if err != nil {
		return nil, dbErr(err)
	}
	var id uuid.UUID
	_, err = pgx.ForEachRow(rows, []any{&id}, func() error {
		out[id] = true
		return nil
	})
	return out, dbErr(err)
}

func (s *RepositoryStore) Readme(ctx context.Context, projectID uuid.UUID) (*domain.Readme, error) {
	var r domain.Readme
	var kind string
	err := s.pool.QueryRow(ctx, repoReadme, projectID).Scan(&r.Markdown, &r.Truncated, &r.WebURL, &r.DefaultBranch, &kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	r.Kind = domain.Kind(kind)
	return &r, nil
}
