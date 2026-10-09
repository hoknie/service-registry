package knowledge

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/knowledge"
)

type SourceStore struct{ pool *pgxpool.Pool }

func NewSourceStore(pool *pgxpool.Pool) *SourceStore { return &SourceStore{pool: pool} }

func scanSource(row pgx.Row) (domain.StoredSource, error) {
	var s domain.StoredSource
	var kind string
	var fp *string
	err := row.Scan(&kind, &s.Forge, &s.URL, &s.APIURL, &s.Path, &s.CredentialsEnc, &s.CredentialsRef, &fp, &s.Heads, &s.DefaultBranch, &s.UpdatedAt)
	s.Kind = domain.SourceKind(kind)
	switch {
	case s.CredentialsEnc != nil:
		s.Credentials = domain.Credentials{Mode: "stored", Fingerprint: fp}
	case s.CredentialsRef != nil:
		s.Credentials = domain.Credentials{Mode: "reference"}
	default:
		s.Credentials = domain.Credentials{Mode: "none"}
	}
	if s.Heads == nil {
		s.Heads = map[string]string{}
	}
	return s, err
}

func (s *SourceStore) Get(ctx context.Context, projectID uuid.UUID) (*domain.StoredSource, error) {
	src, err := scanSource(s.pool.QueryRow(ctx, `
		SELECT s.kind, COALESCE(s.forge, ''), COALESCE(s.url, ''), COALESCE(s.api_url, ''),
			COALESCE(s.path, ''), s.credentials_enc, s.credentials_ref, s.credentials_fingerprint, s.heads,
			COALESCE(s.default_branch, ''), rfc3339(s.updated_at)
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

func (s *SourceStore) Put(ctx context.Context, projectID uuid.UUID, n domain.NewSource) (domain.Source, error) {
	var out domain.StoredSource
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		out, err = scanSource(tx.QueryRow(ctx, `
			INSERT INTO knowledge_sources AS s (project_id, kind, forge, url, api_url, path, credentials_enc,
				credentials_ref, credentials_fingerprint)
			VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), $7, $8, $9)
			ON CONFLICT (project_id)
			DO UPDATE SET kind = EXCLUDED.kind, forge = EXCLUDED.forge, url = EXCLUDED.url,
				api_url = EXCLUDED.api_url, path = EXCLUDED.path,
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
				COALESCE(s.default_branch, ''), rfc3339(s.updated_at)`, projectID, string(n.Kind), n.Forge, n.URL, n.APIURL, n.Path,
			n.CredentialsEnc, n.CredentialsRef, n.Fingerprint, n.Keep))
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

func (s *SourceStore) Delete(ctx context.Context, projectID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM knowledge_sources
		WHERE project_id = $1`, projectID)
	return dbErr(err)
}

func (s *SourceStore) SetHeads(ctx context.Context, projectID uuid.UUID, heads map[string]string, defaultBranch string) error {
	if heads == nil {
		heads = map[string]string{}
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE knowledge_sources
		SET heads = $2, default_branch = NULLIF($3, '')
		WHERE project_id = $1`, projectID, heads, defaultBranch)
	return dbErr(err)
}

func (s *SourceStore) Secrets(ctx context.Context) ([]domain.SourceSecret, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT project_id, credentials_enc
		FROM knowledge_sources
		WHERE credentials_enc IS NOT NULL
		ORDER BY project_id`)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.SourceSecret, error) {
		var x domain.SourceSecret
		err := r.Scan(&x.ProjectID, &x.CredentialsEnc)
		return x, err
	})
	return out, dbErr(err)
}

func (s *SourceStore) ReplaceSecret(ctx context.Context, projectID uuid.UUID, enc string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE knowledge_sources
		SET credentials_enc = $2
		WHERE project_id = $1`, projectID, enc)
	return dbErr(err)
}
