package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type SourceRepository struct{ db *pgtx.DB }

func NewSourceRepository(db *pgtx.DB) *SourceRepository { return &SourceRepository{db: db} }

func scanSource(row pgx.CollectableRow) (knowledgetypes.StoredSource, error) {
	var s knowledgetypes.StoredSource
	var kind string
	err := row.Scan(&kind, &s.Forge, &s.URL, &s.APIURL, &s.Path, &s.CredentialsSecretID,
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
		SELECT s.kind, COALESCE(s.forge, ''), COALESCE(s.url, ''), COALESCE(s.api_url, ''),
			COALESCE(s.path, ''), s.credentials_secret_id, s.heads,
			COALESCE(s.default_branch, ''), s.working_tree, s.include_ignored, rfc3339(s.updated_at)
		FROM knowledge_sources s
		WHERE s.project_id = $1`, projectID)
}

func (s *SourceRepository) Put(ctx context.Context, projectID uuid.UUID, n knowledgetypes.NewSource) (knowledgetypes.Source, error) {
	var out knowledgetypes.StoredSource
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		var err error
		out, err = postgres.One(ctx, tx, constraints, scanSource, `
			INSERT INTO knowledge_sources AS s (project_id, kind, forge, url, api_url, path, credentials_secret_id,
				working_tree, include_ignored)
			VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), $7, $9, $10)
			ON CONFLICT (project_id)
			DO UPDATE SET kind = EXCLUDED.kind, forge = EXCLUDED.forge, url = EXCLUDED.url,
				api_url = EXCLUDED.api_url, path = EXCLUDED.path, working_tree = EXCLUDED.working_tree,
				include_ignored = EXCLUDED.include_ignored,
				credentials_secret_id = CASE
					WHEN $8 AND s.kind = EXCLUDED.kind AND s.url IS NOT DISTINCT FROM EXCLUDED.url THEN s.credentials_secret_id
					ELSE EXCLUDED.credentials_secret_id END,
				heads = CASE
					WHEN s.kind = EXCLUDED.kind AND s.url IS NOT DISTINCT FROM EXCLUDED.url AND s.path IS NOT DISTINCT FROM EXCLUDED.path THEN s.heads
					ELSE '{}' END,
				updated_at = now()
			RETURNING s.kind, COALESCE(s.forge, ''), COALESCE(s.url, ''), COALESCE(s.api_url, ''),
				COALESCE(s.path, ''), s.credentials_secret_id, s.heads,
				COALESCE(s.default_branch, ''), s.working_tree, s.include_ignored, rfc3339(s.updated_at)`, projectID, string(n.Kind),
			n.Forge, n.URL, n.APIURL, n.Path, n.SecretID, n.Keep, n.WorkingTree, n.IncludeIgnored)
		if err != nil {
			return err
		}
		_, err = postgres.Exec(ctx, tx, constraints, `
			INSERT INTO knowledge_settings (project_id)
			VALUES ($1)
			ON CONFLICT (project_id)
			DO UPDATE SET next_run_at = now()`, projectID)
		return err
	})
	return out.Source, postgres.Translate(err, constraints)
}

func (s *SourceRepository) Delete(ctx context.Context, projectID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM knowledge_sources
		WHERE project_id = $1`, projectID)
	return err
}

func (s *SourceRepository) SetHeads(ctx context.Context, projectID uuid.UUID, heads map[string]string, defaultBranch string) error {
	if heads == nil {
		heads = map[string]string{}
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE knowledge_sources
		SET heads = $2, default_branch = NULLIF($3, '')
		WHERE project_id = $1`, projectID, heads, defaultBranch)
	return err
}

func (s *SourceRepository) Legacy(ctx context.Context) ([]knowledgetypes.LegacySource, error) {
	tx := s.db.From(ctx)
	if ok, err := postgres.HasColumn(ctx, tx, knowledgetypes.SourceTable, knowledgetypes.SourceColumnCredentials); err != nil || !ok {
		return nil, err
	}
	return postgres.List(ctx, tx, constraints, func(r pgx.CollectableRow) (knowledgetypes.LegacySource, error) {
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
