package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	forgetypes "svc-registry/internal/feature/forge/types"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

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

func (s *SourceRepoRepository) Find(ctx context.Context, projectID uuid.UUID) (*forgetypes.Repository, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanRepository, `
		SELECT k.full_path, k.web_url, k.description, k.topics, k.languages, k.default_branch, k.archived, k.visibility,
			k.stars, k.license, rfc3339(k.pushed_at), rfc3339(k.synced_at), k.readme_md IS NOT NULL, s.forge
		FROM knowledge_source_repos k
		JOIN knowledge_sources s ON s.project_id = k.project_id AND s.kind = 'remote'
		WHERE k.project_id = $1`, projectID)
}

func scanRepository(row pgx.CollectableRow) (forgetypes.Repository, error) {
	var r forgetypes.Repository
	var vis, kind string
	if err := row.Scan(&r.FullPath, &r.WebURL, &r.Description, &r.Topics, &r.Languages,
		&r.DefaultBranch, &r.Archived, &vis, &r.Stars, &r.License, &r.PushedAt, &r.SyncedAt, &r.HasReadme, &kind); err != nil {
		return r, err
	}
	r.Visibility = forgetypes.Visibility(vis)
	r.Kind = forgetypes.Kind(kind)
	r.Origin = forgetypes.OriginSource
	if r.Topics == nil {
		r.Topics = []string{}
	}
	if r.Languages == nil {
		r.Languages = map[string]float64{}
	}
	return r, nil
}

func (s *SourceRepoRepository) Readme(ctx context.Context, projectID uuid.UUID) (*forgetypes.Readme, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(row pgx.CollectableRow) (forgetypes.Readme, error) {
		var r forgetypes.Readme
		var kind string
		err := row.Scan(&r.Markdown, &r.Truncated, &r.WebURL, &r.DefaultBranch, &kind)
		r.Kind = forgetypes.Kind(kind)
		return r, err
	}, `
		SELECT k.readme_md, k.readme_truncated, k.web_url, k.default_branch, s.forge
		FROM knowledge_source_repos k
		JOIN knowledge_sources s ON s.project_id = k.project_id AND s.kind = 'remote'
		WHERE k.project_id = $1
			AND k.readme_md IS NOT NULL`, projectID)
}
