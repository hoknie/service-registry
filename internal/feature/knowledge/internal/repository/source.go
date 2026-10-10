package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/platform/postgres"
)

type Sources struct{ db *postgres.DB }

func NewSources(db *postgres.DB) *Sources { return &Sources{db: db} }

func scanSource(row pgx.Row) (knowledge.StoredSource, error) {
	var s knowledge.StoredSource
	var kind string
	var fp *string
	err := row.Scan(&kind, &s.Forge, &s.URL, &s.APIURL, &s.Path, &s.CredentialsEnc, &s.CredentialsRef, &fp, &s.Heads, &s.DefaultBranch,
		&s.WorkingTree, &s.IncludeIgnored, &s.UpdatedAt)
	s.Kind = knowledge.SourceKind(kind)
	switch {
	case s.CredentialsEnc != nil:
		s.Credentials = knowledge.Credentials{Mode: "stored", Fingerprint: fp}
	case s.CredentialsRef != nil:
		s.Credentials = knowledge.Credentials{Mode: "reference"}
	default:
		s.Credentials = knowledge.Credentials{Mode: "none"}
	}
	if s.Heads == nil {
		s.Heads = map[string]string{}
	}
	return s, err
}

func (s *Sources) Get(ctx context.Context, projectID uuid.UUID) (*knowledge.StoredSource, error) {
	src, err := scanSource(s.db.From(ctx).QueryRow(ctx, `
		SELECT s.kind, COALESCE(s.forge, ''), COALESCE(s.url, ''), COALESCE(s.api_url, ''),
			COALESCE(s.path, ''), s.credentials_enc, s.credentials_ref, s.credentials_fingerprint, s.heads,
			COALESCE(s.default_branch, ''), s.working_tree, s.include_ignored, rfc3339(s.updated_at)
		FROM knowledge_sources s
		WHERE s.project_id = $1`, projectID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &src, nil
}

func (s *Sources) Put(ctx context.Context, projectID uuid.UUID, n knowledge.NewSource) (knowledge.Source, error) {
	var out knowledge.StoredSource
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		var err error
		out, err = scanSource(tx.QueryRow(ctx, `
			INSERT INTO knowledge_sources AS s (project_id, kind, forge, url, api_url, path, credentials_enc,
				credentials_ref, credentials_fingerprint, working_tree, include_ignored)
			VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), $7, $8, $9, $11, $12)
			ON CONFLICT (project_id)
			DO UPDATE SET kind = EXCLUDED.kind, forge = EXCLUDED.forge, url = EXCLUDED.url,
				api_url = EXCLUDED.api_url, path = EXCLUDED.path, working_tree = EXCLUDED.working_tree,
				include_ignored = EXCLUDED.include_ignored,
				credentials_enc = CASE
					WHEN $10 AND s.kind = EXCLUDED.kind AND s.url IS NOT DISTINCT FROM EXCLUDED.url THEN s.credentials_enc
					ELSE EXCLUDED.credentials_enc END,
				credentials_ref = CASE
					WHEN $10 AND s.kind = EXCLUDED.kind AND s.url IS NOT DISTINCT FROM EXCLUDED.url THEN s.credentials_ref
					ELSE EXCLUDED.credentials_ref END,
				credentials_fingerprint = CASE
					WHEN $10 AND s.kind = EXCLUDED.kind AND s.url IS NOT DISTINCT FROM EXCLUDED.url THEN s.credentials_fingerprint
					ELSE EXCLUDED.credentials_fingerprint END,
				heads = CASE
					WHEN s.kind = EXCLUDED.kind AND s.url IS NOT DISTINCT FROM EXCLUDED.url AND s.path IS NOT DISTINCT FROM EXCLUDED.path THEN s.heads
					ELSE '{}' END,
				updated_at = now()
			RETURNING s.kind, COALESCE(s.forge, ''), COALESCE(s.url, ''), COALESCE(s.api_url, ''),
				COALESCE(s.path, ''), s.credentials_enc, s.credentials_ref, s.credentials_fingerprint, s.heads,
				COALESCE(s.default_branch, ''), s.working_tree, s.include_ignored, rfc3339(s.updated_at)`, projectID, string(n.Kind),
			n.Forge, n.URL, n.APIURL, n.Path, n.CredentialsEnc, n.CredentialsRef, n.Fingerprint, n.Keep, n.WorkingTree, n.IncludeIgnored))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO knowledge_settings (project_id)
			VALUES ($1)
			ON CONFLICT (project_id)
			DO UPDATE SET next_run_at = now()`, projectID)
		return err
	})
	return out.Source, dbErr(err)
}

func (s *Sources) Delete(ctx context.Context, projectID uuid.UUID) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM knowledge_sources
		WHERE project_id = $1`, projectID)
	return dbErr(err)
}

func (s *Sources) SetHeads(ctx context.Context, projectID uuid.UUID, heads map[string]string, defaultBranch string) error {
	if heads == nil {
		heads = map[string]string{}
	}
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE knowledge_sources
		SET heads = $2, default_branch = NULLIF($3, '')
		WHERE project_id = $1`, projectID, heads, defaultBranch)
	return dbErr(err)
}

func (s *Sources) Secrets(ctx context.Context) ([]knowledge.SourceSecret, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT project_id, credentials_enc
		FROM knowledge_sources
		WHERE credentials_enc IS NOT NULL
		ORDER BY project_id`)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (knowledge.SourceSecret, error) {
		var x knowledge.SourceSecret
		err := r.Scan(&x.ProjectID, &x.CredentialsEnc)
		return x, err
	})
	return out, dbErr(err)
}

func (s *Sources) ReplaceSecret(ctx context.Context, projectID uuid.UUID, enc string) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE knowledge_sources
		SET credentials_enc = $2
		WHERE project_id = $1`, projectID, enc)
	return dbErr(err)
}
