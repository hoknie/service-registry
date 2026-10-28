package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type SourceRepository struct{ db *pgtx.DB }

func NewSourceRepository(db *pgtx.DB) *SourceRepository { return &SourceRepository{db: db} }

func scanSource(row pgx.CollectableRow) (knowledgetypes.StoredSource, error) {
	var s knowledgetypes.StoredSource
	var kind string
	err := row.Scan(&kind, &s.Hosting, &s.URL, &s.APIURL, &s.Path, &s.CredentialsSecretID,
		&s.Heads, &s.DefaultBranch,
		&s.WorkingTree, &s.IncludeIgnored, &s.UpdatedAt)
	s.Kind = knowledgetypes.SourceKind(kind)
	s.Credentials = catalogtypes.Credentials{Kind: catalogtypes.CredentialsNone}
	if s.CredentialsSecretID != nil {
		s.Credentials = catalogtypes.Credentials{Kind: catalogtypes.CredentialsSecret, SecretID: s.CredentialsSecretID}
	}
	if s.Heads == nil {
		s.Heads = map[string]string{}
	}
	return s, err
}

func (s *SourceRepository) Get(ctx context.Context, projectID uuid.UUID) (*knowledgetypes.StoredSource, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanSource, `
		SELECT s.kind, COALESCE(s.hosting, ''), COALESCE(s.url, ''), COALESCE(s.api_url, ''),
			COALESCE(s.path, ''), s.credentials_secret_id, s.heads,
			COALESCE(s.default_branch, ''), s.working_tree, s.include_ignored, rfc3339(s.updated_at)
		FROM knowledge_sources s
		WHERE s.project_id = $1`, projectID)
}

func (s *SourceRepository) Upsert(ctx context.Context, projectID uuid.UUID, n knowledgetypes.NewSource) (knowledgetypes.Source, error) {
	out, err := postgres.One(ctx, s.db.From(ctx), constraints, scanSource, `
		INSERT INTO knowledge_sources AS s (project_id, kind, hosting, url, api_url, path, credentials_secret_id,
			working_tree, include_ignored)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), $7, $9, $10)
		ON CONFLICT (project_id)
		DO UPDATE SET kind = EXCLUDED.kind, hosting = EXCLUDED.hosting, url = EXCLUDED.url,
			api_url = EXCLUDED.api_url, path = EXCLUDED.path, working_tree = EXCLUDED.working_tree,
			include_ignored = EXCLUDED.include_ignored,
			credentials_secret_id = CASE
				WHEN $8 AND s.kind = EXCLUDED.kind AND s.url IS NOT DISTINCT FROM EXCLUDED.url THEN s.credentials_secret_id
				ELSE EXCLUDED.credentials_secret_id END,
			heads = CASE
				WHEN s.kind = EXCLUDED.kind AND s.url IS NOT DISTINCT FROM EXCLUDED.url AND s.path IS NOT DISTINCT FROM EXCLUDED.path THEN s.heads
				ELSE '{}' END,
			updated_at = now()
		RETURNING s.kind, COALESCE(s.hosting, ''), COALESCE(s.url, ''), COALESCE(s.api_url, ''),
			COALESCE(s.path, ''), s.credentials_secret_id, s.heads,
			COALESCE(s.default_branch, ''), s.working_tree, s.include_ignored, rfc3339(s.updated_at)`, projectID, string(n.Kind),
		n.Hosting, n.URL, n.APIURL, n.Path, n.SecretID, n.Keep, n.WorkingTree, n.IncludeIgnored)
	return out.Source, err
}

func (s *SourceRepository) Delete(ctx context.Context, projectID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM knowledge_sources
		WHERE project_id = $1`, projectID)
	return err
}

func (s *SourceRepository) SetHeads(ctx context.Context, projectID uuid.UUID, heads map[string]string, defaultBranch string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE knowledge_sources
		SET heads = $2, default_branch = NULLIF($3, '')
		WHERE project_id = $1`, projectID, heads, defaultBranch)
	return err
}

func (s *SourceRepository) HasLegacyCredentials(ctx context.Context) (bool, error) {
	return postgres.HasColumn(ctx, s.db.From(ctx), knowledgetypes.SourceTable, knowledgetypes.SourceColumnCredentials)
}

func (s *SourceRepository) Legacy(ctx context.Context) ([]knowledgetypes.LegacySource, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (knowledgetypes.LegacySource, error) {
		var x knowledgetypes.LegacySource
		err := r.Scan(&x.ProjectID, &x.ParentID, &x.Slug, &x.Enc, &x.Ref)
		return x, err
	}, `
		SELECT s.project_id, n.parent_id, n.slug, s.credentials_enc, s.credentials_ref
		FROM knowledge_sources s
		JOIN nodes n ON n.id = s.project_id
		WHERE s.credentials_secret_id IS NULL
			AND (s.credentials_enc IS NOT NULL OR s.credentials_ref IS NOT NULL)
		ORDER BY s.project_id`)
}

func (s *SourceRepository) AdoptSecret(ctx context.Context, projectID, secret uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE knowledge_sources
		SET credentials_secret_id = $2, credentials_enc = NULL, credentials_ref = NULL, credentials_fingerprint = NULL,
			updated_at = now()
		WHERE project_id = $1`, projectID, secret)
}

func (s *SourceRepository) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return postgres.Pairs[uuid.UUID, int64](ctx, s.db.From(ctx), constraints, `
		SELECT credentials_secret_id, count(*)
		FROM knowledge_sources
		WHERE credentials_secret_id = ANY($1)
		GROUP BY credentials_secret_id`, secrets)
}

type SourceRepoRepository struct{ db *pgtx.DB }

func NewSourceRepoRepository(db *pgtx.DB) *SourceRepoRepository {
	return &SourceRepoRepository{db: db}
}

func (s *SourceRepoRepository) ChangedAt(ctx context.Context, projectID uuid.UUID) (*time.Time, error) {
	at, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[*time.Time], `
		SELECT source_updated_at
		FROM knowledge_source_repos
		WHERE project_id = $1`, projectID)
	if err != nil || at == nil {
		return nil, err
	}
	return *at, nil
}

func (s *SourceRepoRepository) Upsert(ctx context.Context, projectID uuid.UUID, rec knowledgetypes.SourceRepoRecord) error {
	r := rec.Repo
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO knowledge_source_repos AS k (project_id, full_path, web_url, description, topics, default_branch,
			archived, visibility, stars, license, pushed_at, source_updated_at, readme_md, readme_truncated, languages)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $14, COALESCE($15, false), COALESCE($16, '{}'::jsonb))
		ON CONFLICT (project_id)
		DO UPDATE SET full_path = EXCLUDED.full_path, web_url = EXCLUDED.web_url, description = EXCLUDED.description,
			topics = EXCLUDED.topics, default_branch = EXCLUDED.default_branch, archived = EXCLUDED.archived,
			visibility = EXCLUDED.visibility, stars = EXCLUDED.stars, pushed_at = EXCLUDED.pushed_at,
			license = CASE WHEN $13 THEN EXCLUDED.license ELSE COALESCE(EXCLUDED.license, k.license) END,
			source_updated_at = CASE WHEN $13 THEN EXCLUDED.source_updated_at ELSE k.source_updated_at END,
			readme_md = CASE WHEN $13 THEN EXCLUDED.readme_md ELSE k.readme_md END,
			readme_truncated = CASE WHEN $13 THEN EXCLUDED.readme_truncated ELSE k.readme_truncated END,
			languages = CASE WHEN $13 THEN EXCLUDED.languages ELSE k.languages END,
			synced_at = now()`, projectID, r.FullPath, r.WebURL, r.Description, r.Topics, rec.DefaultBranch, r.Archived,
		string(r.Visibility), r.Stars, rec.License, r.PushedAt, rec.ChangedAt, rec.Details, rec.Readme, rec.Truncated, rec.Languages)
	return err
}

func (s *SourceRepoRepository) Find(ctx context.Context, projectID uuid.UUID) (*reposynctypes.Repository, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanRepository, `
		SELECT k.full_path, k.web_url, k.description, k.topics, k.languages, k.default_branch, k.archived, k.visibility,
			k.stars, k.license, rfc3339(k.pushed_at), rfc3339(k.synced_at), k.readme_md IS NOT NULL, s.hosting
		FROM knowledge_source_repos k
		JOIN knowledge_sources s ON s.project_id = k.project_id AND s.kind = 'remote'
		WHERE k.project_id = $1`, projectID)
}

func scanRepository(row pgx.CollectableRow) (reposynctypes.Repository, error) {
	var r reposynctypes.Repository
	var vis, kind string
	if err := row.Scan(&r.FullPath, &r.WebURL, &r.Description, &r.Topics, &r.Languages,
		&r.DefaultBranch, &r.Archived, &vis, &r.Stars, &r.License, &r.PushedAt, &r.SyncedAt, &r.HasReadme, &kind); err != nil {
		return r, err
	}
	r.Visibility = reposynctypes.Visibility(vis)
	r.Kind = reposynctypes.Kind(kind)
	r.Origin = reposynctypes.OriginSource
	if r.Topics == nil {
		r.Topics = []string{}
	}
	if r.Languages == nil {
		r.Languages = map[string]float64{}
	}
	return r, nil
}

func (s *SourceRepoRepository) Readme(ctx context.Context, projectID uuid.UUID) (*reposynctypes.Readme, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(row pgx.CollectableRow) (reposynctypes.Readme, error) {
		var r reposynctypes.Readme
		var kind string
		err := row.Scan(&r.Markdown, &r.Truncated, &r.WebURL, &r.DefaultBranch, &kind)
		r.Kind = reposynctypes.Kind(kind)
		return r, err
	}, `
		SELECT k.readme_md, k.readme_truncated, k.web_url, k.default_branch, s.hosting
		FROM knowledge_source_repos k
		JOIN knowledge_sources s ON s.project_id = k.project_id AND s.kind = 'remote'
		WHERE k.project_id = $1
			AND k.readme_md IS NOT NULL`, projectID)
}
