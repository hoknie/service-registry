package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/platform/postgres"
)

type Indexes struct{ db *postgres.DB }

func NewIndexes(db *postgres.DB) *Indexes { return &Indexes{db: db} }

func (s *Indexes) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	if _, err := s.db.From(ctx).Exec(ctx, `
		INSERT INTO knowledge_index_state (project_id)
		SELECT DISTINCT project_id
		FROM knowledge_snapshots
		ON CONFLICT
		DO NOTHING`); err != nil {
		return nil, dbErr(err)
	}
	rows, err := s.db.From(ctx).Query(ctx, `
		UPDATE knowledge_index_state s
		SET lease_until = now() + make_interval(secs => $2)
		WHERE s.project_id IN (
			SELECT k.project_id
			FROM knowledge_index_state k
			WHERE k.next_run_at <= now()
				AND (k.lease_until IS NULL OR k.lease_until < now())
			ORDER BY k.next_run_at
			LIMIT $1
			FOR UPDATE OF k SKIP LOCKED
		)
		RETURNING s.project_id`, limit, float64(leaseSecs))
	if err != nil {
		return nil, dbErr(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	return ids, dbErr(err)
}

func (s *Indexes) Extend(ctx context.Context, project uuid.UUID, leaseSecs int) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE knowledge_index_state
		SET lease_until = now() + make_interval(secs => $2)
		WHERE project_id = $1`, project, float64(leaseSecs))
	return dbErr(err)
}

func (s *Indexes) Release(ctx context.Context, project uuid.UUID, afterSecs uint32, failure string) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE knowledge_index_state
		SET lease_until = NULL, next_run_at = now() + make_interval(secs => $2), failure = NULLIF($3, '')
		WHERE project_id = $1`, project, float64(afterSecs), failure)
	return dbErr(err)
}

func (s *Indexes) State(ctx context.Context, project uuid.UUID) (knowledge.IndexState, error) {
	var st knowledge.IndexState
	err := s.db.From(ctx).QueryRow(ctx, `
		SELECT COALESCE(engine, ''), COALESCE(model, ''), COALESCE(fingerprint, '')
		FROM knowledge_index_state
		WHERE project_id = $1`, project).Scan(&st.Engine, &st.Model, &st.Fingerprint)
	if err == pgx.ErrNoRows {
		return knowledge.IndexState{}, nil
	}
	return st, dbErr(err)
}

func (s *Indexes) Synced(ctx context.Context, project uuid.UUID, st knowledge.IndexState) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE knowledge_index_state
		SET engine = $2, model = $3, fingerprint = $4, synced_at = now()
		WHERE project_id = $1`, project, st.Engine, st.Model, st.Fingerprint)
	return dbErr(err)
}

func (s *Indexes) Files(ctx context.Context, project uuid.UUID) ([]knowledge.IndexFile, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (branch) id, branch, commit_sha
			FROM knowledge_snapshots
			WHERE project_id = $1
				AND status <> 'failed'
			ORDER BY branch, collected_at DESC, id DESC
		)
		SELECT l.branch, l.commit_sha, f.path, f.kind, f.sha256
		FROM latest l
		JOIN knowledge_files f ON f.snapshot_id = l.id
		WHERE f.sha256 IS NOT NULL
		ORDER BY l.branch, f.path`, project)
	if err != nil {
		return nil, dbErr(err)
	}
	files, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (knowledge.IndexFile, error) {
		var f knowledge.IndexFile
		var kind string
		err := r.Scan(&f.Branch, &f.Commit, &f.Path, &kind, &f.SHA256)
		f.Kind = knowledge.Kind(kind)
		return f, err
	})
	return files, dbErr(err)
}

func (s *Indexes) Missing(ctx context.Context, project uuid.UUID, model string) ([]knowledge.BlobContent, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (branch) id, branch, commit_sha
			FROM knowledge_snapshots
			WHERE project_id = $1
				AND status <> 'failed'
			ORDER BY branch, collected_at DESC, id DESC
		)
		SELECT DISTINCT b.sha256, b.content
		FROM latest l
		JOIN knowledge_files f ON f.snapshot_id = l.id
		JOIN knowledge_blobs b ON b.sha256 = f.sha256
		WHERE NOT EXISTS (
			SELECT 1
			FROM knowledge_embeddings e
			WHERE e.sha256 = b.sha256
				AND e.model = $2
		)`, project, model)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (knowledge.BlobContent, error) {
		var b knowledge.BlobContent
		err := r.Scan(&b.SHA256, &b.Content)
		return b, err
	})
	return out, dbErr(err)
}

func (s *Indexes) Contents(ctx context.Context, shas [][]byte) (map[string]string, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT sha256, content
		FROM knowledge_blobs
		WHERE sha256 = ANY($1)`, shas)
	if err != nil {
		return nil, dbErr(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var sha []byte
		var content string
		if err := rows.Scan(&sha, &content); err != nil {
			return nil, dbErr(err)
		}
		out[string(sha)] = content
	}
	return out, dbErr(rows.Err())
}

func (s *Indexes) Save(ctx context.Context, sha256 []byte, model string, chunks []knowledge.StoredChunk) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return dbErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		DELETE FROM knowledge_embeddings
		WHERE sha256 = $1
			AND model = $2`, sha256, model); err != nil {
		return dbErr(err)
	}
	for _, c := range chunks {
		if _, err := tx.Exec(ctx, `
			INSERT INTO knowledge_embeddings (sha256, model, ord, chunk_start, chunk_end, vector)
			VALUES ($1, $2, $3, $4, $5, $6)`, sha256, model, c.Ord, c.Span.Start, c.Span.End, c.Vector); err != nil {
			return dbErr(err)
		}
	}
	return dbErr(tx.Commit(ctx))
}

func (s *Indexes) Chunks(ctx context.Context, shas [][]byte, model string) (map[string][]knowledge.StoredChunk, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT sha256, ord, chunk_start, chunk_end, vector
		FROM knowledge_embeddings
		WHERE sha256 = ANY($1)
			AND model = $2
		ORDER BY sha256, ord`, shas, model)
	if err != nil {
		return nil, dbErr(err)
	}
	defer rows.Close()
	out := map[string][]knowledge.StoredChunk{}
	for rows.Next() {
		var sha []byte
		var c knowledge.StoredChunk
		if err := rows.Scan(&sha, &c.Ord, &c.Span.Start, &c.Span.End, &c.Vector); err != nil {
			return nil, dbErr(err)
		}
		out[string(sha)] = append(out[string(sha)], c)
	}
	return out, dbErr(rows.Err())
}

func (s *Indexes) Counts(ctx context.Context, model string) (pending, indexed int, err error) {
	err = s.db.From(ctx).QueryRow(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (project_id, branch) id
			FROM knowledge_snapshots
			WHERE status <> 'failed'
			ORDER BY project_id, branch, collected_at DESC, id DESC
		)
		SELECT count(*) FILTER (WHERE NOT x.done), count(*) FILTER (WHERE x.done)
		FROM (
			SELECT EXISTS (
				SELECT 1
				FROM knowledge_embeddings e
				WHERE e.sha256 = f.sha256
					AND e.model = $1
			) AS done
			FROM latest l
			JOIN knowledge_files f ON f.snapshot_id = l.id
			WHERE f.sha256 IS NOT NULL
		) x`, model).Scan(&pending, &indexed)
	return pending, indexed, dbErr(err)
}

func (s *Indexes) Indexed(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT project_id
		FROM knowledge_index_state
		ORDER BY project_id`)
	if err != nil {
		return nil, dbErr(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	return ids, dbErr(err)
}

func (s *Indexes) Forget(ctx context.Context, keep []uuid.UUID) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE knowledge_index_state
		SET fingerprint = NULL
		WHERE NOT (project_id = ANY($1))`, keep)
	return dbErr(err)
}
