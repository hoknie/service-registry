package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type IndexRepository struct{ db *pgtx.DB }

func NewIndexRepository(db *pgtx.DB) *IndexRepository { return &IndexRepository{db: db} }

func (s *IndexRepository) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	if _, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO knowledge_index_state (project_id)
		SELECT DISTINCT project_id
		FROM knowledge_snapshots
		ON CONFLICT
		DO NOTHING`); err != nil {
		return nil, err
	}
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
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
}

func (s *IndexRepository) Extend(ctx context.Context, project uuid.UUID, leaseSecs int) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE knowledge_index_state
		SET lease_until = now() + make_interval(secs => $2)
		WHERE project_id = $1`, project, float64(leaseSecs))
	return err
}

func (s *IndexRepository) Release(ctx context.Context, project uuid.UUID, afterSecs uint32, failure string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE knowledge_index_state
		SET lease_until = NULL, next_run_at = now() + make_interval(secs => $2), failure = NULLIF($3, '')
		WHERE project_id = $1`, project, float64(afterSecs), failure)
	return err
}

func (s *IndexRepository) State(ctx context.Context, project uuid.UUID) (knowledgetypes.IndexState, error) {
	st, err := postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (knowledgetypes.IndexState, error) {
		var st knowledgetypes.IndexState
		err := r.Scan(&st.Engine, &st.Model, &st.Fingerprint)
		return st, err
	}, `
		SELECT COALESCE(engine, ''), COALESCE(model, ''), COALESCE(fingerprint, '')
		FROM knowledge_index_state
		WHERE project_id = $1`, project)
	if err != nil || st == nil {
		return knowledgetypes.IndexState{}, err
	}
	return *st, nil
}

func (s *IndexRepository) Synced(ctx context.Context, project uuid.UUID, st knowledgetypes.IndexState) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE knowledge_index_state
		SET engine = $2, model = $3, fingerprint = $4, synced_at = now()
		WHERE project_id = $1`, project, st.Engine, st.Model, st.Fingerprint)
	return err
}

func (s *IndexRepository) Files(ctx context.Context, project uuid.UUID) ([]knowledgetypes.IndexFile, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(indexFileRow.file), `
		WITH latest AS (
			SELECT DISTINCT ON (ref_kind, branch) id, CASE WHEN ref_kind = 'tag' THEN 'tag:' || branch ELSE branch END AS branch, commit_sha
			FROM knowledge_snapshots
			WHERE project_id = $1
				AND status <> 'failed'
			ORDER BY ref_kind, branch, collected_at DESC, id DESC
		)
		SELECT l.branch, l.commit_sha, f.path, f.kind, f.sha256
		FROM latest l
		JOIN knowledge_files f ON f.snapshot_id = l.id
		WHERE f.sha256 IS NOT NULL
		ORDER BY l.branch, f.path`, project)
}

func (s *IndexRepository) Missing(ctx context.Context, project uuid.UUID, model string) ([]knowledgetypes.BlobContent, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (knowledgetypes.BlobContent, error) {
		var b knowledgetypes.BlobContent
		err := r.Scan(&b.SHA256, &b.Content)
		return b, err
	}, `
		WITH latest AS (
			SELECT DISTINCT ON (ref_kind, branch) id, CASE WHEN ref_kind = 'tag' THEN 'tag:' || branch ELSE branch END AS branch, commit_sha
			FROM knowledge_snapshots
			WHERE project_id = $1
				AND status <> 'failed'
			ORDER BY ref_kind, branch, collected_at DESC, id DESC
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
}

func (s *IndexRepository) Contents(ctx context.Context, shas [][]byte) (map[string]string, error) {
	rows, err := postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowToStructByPos[blobRow], `
		SELECT sha256, content
		FROM knowledge_blobs
		WHERE sha256 = ANY($1)`, shas)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range rows {
		out[string(r.SHA256)] = r.Content
	}
	return out, nil
}

func (s *IndexRepository) ReplaceEmbeddings(ctx context.Context, sha256 []byte, model string, chunks []knowledgetypes.StoredChunk) error {
	tx := s.db.From(ctx)
	if _, err := postgres.Exec(ctx, tx, constraints, `
		DELETE FROM knowledge_embeddings
		WHERE sha256 = $1
			AND model = $2`, sha256, model); err != nil {
		return err
	}
	for _, c := range chunks {
		if _, err := postgres.Exec(ctx, tx, constraints, `
			INSERT INTO knowledge_embeddings (sha256, model, ord, chunk_start, chunk_end, vector)
			VALUES ($1, $2, $3, $4, $5, $6)`, sha256, model, c.Ord, c.Span.Start, c.Span.End, c.Vector); err != nil {
			return err
		}
	}
	return nil
}

func (s *IndexRepository) Chunks(ctx context.Context, shas [][]byte, model string) (map[string][]knowledgetypes.StoredChunk, error) {
	rows, err := postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (chunkRow, error) {
		var c chunkRow
		err := r.Scan(&c.SHA256, &c.Chunk.Ord, &c.Chunk.Span.Start, &c.Chunk.Span.End, &c.Chunk.Vector)
		return c, err
	}, `
		SELECT sha256, ord, chunk_start, chunk_end, vector
		FROM knowledge_embeddings
		WHERE sha256 = ANY($1)
			AND model = $2
		ORDER BY sha256, ord`, shas, model)
	if err != nil {
		return nil, err
	}
	out := map[string][]knowledgetypes.StoredChunk{}
	for _, r := range rows {
		out[string(r.SHA256)] = append(out[string(r.SHA256)], r.Chunk)
	}
	return out, nil
}

func (s *IndexRepository) Counts(ctx context.Context, model string) (pending, indexed int, err error) {
	c, err := postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowToStructByPos[countsRow], `
		WITH latest AS (
			SELECT DISTINCT ON (project_id, ref_kind, branch) id
			FROM knowledge_snapshots
			WHERE status <> 'failed'
			ORDER BY project_id, ref_kind, branch, collected_at DESC, id DESC
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
		) x`, model)
	return c.Pending, c.Indexed, err
}

func (s *IndexRepository) Indexed(ctx context.Context) ([]uuid.UUID, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		SELECT project_id
		FROM knowledge_index_state
		ORDER BY project_id`)
}

func (s *IndexRepository) Forget(ctx context.Context, keep []uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE knowledge_index_state
		SET fingerprint = NULL
		WHERE NOT (project_id = ANY($1))`, keep)
	return err
}
