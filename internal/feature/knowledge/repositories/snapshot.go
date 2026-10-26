package repositories

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type SnapshotRepository struct{ db *pgtx.DB }

func NewSnapshotRepository(db *pgtx.DB) *SnapshotRepository { return &SnapshotRepository{db: db} }

func lastSnapshot(ctx context.Context, q pgtx.Querier, projectID uuid.UUID, kind knowledgetypes.RefKind, name string, withFailed bool) (*knowledgetypes.Snapshot, error) {
	return postgres.Find(ctx, q, constraints, postgres.Struct(snapshotRow.snapshot), `
		SELECT s.id, s.project_id, s.branch, s.ref_kind, s.commit_sha, s.status, s.error_code, s.files, s.bytes,
			s.skipped, s.truncated, rfc3339(s.collected_at) AS collected_at,
			(SELECT count(*) FROM knowledge_files f WHERE f.snapshot_id = s.id AND f.skip_reason = 'limit')::int AS limited,
			(SELECT count(*) FROM knowledge_files f WHERE f.snapshot_id = s.id AND f.skip_reason = 'too_large')::int AS too_large
		FROM knowledge_snapshots s
		WHERE s.project_id = $1
			AND s.ref_kind = $2
			AND s.branch = $3
			AND ($4 OR s.status <> 'failed')
		ORDER BY s.collected_at DESC, s.id DESC
		LIMIT 1`, projectID, string(kind), name, withFailed)
}

func (s *SnapshotRepository) Branches(ctx context.Context, projectID uuid.UUID) ([]knowledgetypes.Candidate, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (knowledgetypes.Candidate, error) {
		var c knowledgetypes.Candidate
		err := r.Scan(&c.Name, &c.HeadSHA, &c.Gone)
		return c, err
	}, `
		SELECT name, COALESCE(head_sha, '') AS head_sha, gone_at IS NOT NULL AS gone
		FROM branches
		WHERE project_id = $1
		ORDER BY name`, projectID)
}

func (s *SnapshotRepository) LastAttempt(ctx context.Context, projectID uuid.UUID, kind knowledgetypes.RefKind, name string) (*knowledgetypes.Snapshot, error) {
	return lastSnapshot(ctx, s.db.From(ctx), projectID, kind, name, true)
}

func (s *SnapshotRepository) Latest(ctx context.Context, projectID uuid.UUID, kind knowledgetypes.RefKind, name string) (*knowledgetypes.Snapshot, error) {
	return lastSnapshot(ctx, s.db.From(ctx), projectID, kind, name, false)
}

func (s *SnapshotRepository) Known(ctx context.Context, projectID uuid.UUID, shas []string) (map[string]knowledgetypes.Known, error) {
	out := map[string]knowledgetypes.Known{}
	if len(shas) == 0 {
		return out, nil
	}
	found, err := postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowToStructByPos[knownRow], `
		SELECT DISTINCT ON (f.git_blob_sha) f.git_blob_sha, b.sha256, b.content
		FROM knowledge_files f
		JOIN knowledge_snapshots s ON s.id = f.snapshot_id
		JOIN knowledge_blobs b ON b.sha256 = f.sha256
		WHERE s.project_id = $1
			AND f.git_blob_sha = ANY($2)`, projectID, shas)
	if err != nil {
		return nil, err
	}
	for _, r := range found {
		var k knowledgetypes.Known
		copy(k.SHA256[:], r.SHA256)
		k.Content = []byte(r.Content)
		out[r.Git] = k
	}
	return out, nil
}

func (s *SnapshotRepository) Resolve(ctx context.Context, projectID uuid.UUID, kind knowledgetypes.RefKind, name string, commit *string) (*knowledgetypes.Snapshot, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, postgres.Struct(snapshotRow.snapshot), `
		SELECT s.id, s.project_id, s.branch, s.ref_kind, s.commit_sha, s.status, s.error_code, s.files, s.bytes,
			s.skipped, s.truncated, rfc3339(s.collected_at) AS collected_at,
			(SELECT count(*) FROM knowledge_files f WHERE f.snapshot_id = s.id AND f.skip_reason = 'limit')::int AS limited,
			(SELECT count(*) FROM knowledge_files f WHERE f.snapshot_id = s.id AND f.skip_reason = 'too_large')::int AS too_large
		FROM knowledge_snapshots s
		WHERE s.project_id = $1
			AND s.ref_kind = $2
			AND s.branch = $3
			AND ($4::text IS NULL OR s.commit_sha = $4)
			AND s.status <> 'failed'
		ORDER BY s.collected_at DESC, s.id DESC
		LIMIT 1`, projectID, string(kind), name, commit)
}

func scanInfo(r pgx.CollectableRow, extra ...any) (knowledgetypes.FileInfo, error) {
	var f knowledgetypes.FileInfo
	var kind string
	var skip *string
	var meta []byte
	if err := r.Scan(append([]any{&f.Path, &kind, &f.Bytes, &skip, &meta}, extra...)...); err != nil {
		return f, err
	}
	f.Kind = knowledgetypes.Kind(kind)
	if skip != nil {
		f.Skip = new(knowledgetypes.SkipReason(*skip))
	}
	if err := json.Unmarshal(meta, &f.Meta); err != nil {
		return f, err
	}
	return f, nil
}

func (s *SnapshotRepository) Files(ctx context.Context, snapshotID uuid.UUID) ([]knowledgetypes.FileInfo, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (knowledgetypes.FileInfo, error) { return scanInfo(r) }, `
		SELECT path, kind, bytes, skip_reason, meta
		FROM knowledge_files
		WHERE snapshot_id = $1
		ORDER BY path COLLATE "C"`, snapshotID)
}

func (s *SnapshotRepository) File(ctx context.Context, snapshotID uuid.UUID, path string) (*knowledgetypes.FileContent, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (knowledgetypes.FileContent, error) {
		var f knowledgetypes.FileContent
		var err error
		f.FileInfo, err = scanInfo(r, &f.Content)
		return f, err
	}, `
		SELECT f.path, f.kind, f.bytes, f.skip_reason, f.meta, b.content
		FROM knowledge_files f
		LEFT JOIN knowledge_blobs b ON b.sha256 = f.sha256
		WHERE f.snapshot_id = $1
			AND f.path = $2`, snapshotID, path)
}

func (s *SnapshotRepository) Contents(ctx context.Context, snapshotID uuid.UUID, paths []string) (map[string]string, error) {
	out := map[string]string{}
	if len(paths) == 0 {
		return out, nil
	}
	rows, err := postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowToStructByPos[contentRow], `
		SELECT f.path, b.content
		FROM knowledge_files f
		JOIN knowledge_blobs b ON b.sha256 = f.sha256
		WHERE f.snapshot_id = $1
			AND f.path = ANY($2)`, snapshotID, paths)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.Path] = r.Content
	}
	return out, nil
}

func (s *SnapshotRepository) PruneBlobs(ctx context.Context) (int64, error) {
	return postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM knowledge_blobs b
		WHERE NOT EXISTS (SELECT 1 FROM knowledge_files f WHERE f.sha256 = b.sha256)`)
}

func (s *SnapshotRepository) ProjectsWithDocs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]bool{}, nil
	}
	return postgres.Set[uuid.UUID](ctx, s.db.From(ctx), constraints, `
		SELECT DISTINCT project_id
		FROM knowledge_snapshots
		WHERE project_id = ANY($1)
			AND status <> 'failed'`, ids)
}
