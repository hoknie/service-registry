package repositories

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

type SnapshotRepository struct{ db *postgres.DB }

func NewSnapshotRepository(db *postgres.DB) *SnapshotRepository { return &SnapshotRepository{db: db} }

func lastSnapshot(ctx context.Context, q querier, projectID uuid.UUID, branch string, withFailed bool) (*knowledgetypes.Snapshot, error) {
	return oneSnapshot(ctx, q, `
		SELECT s.id, s.project_id, s.branch, s.commit_sha, s.status, s.error_code, s.files, s.bytes,
			s.skipped, s.truncated, rfc3339(s.collected_at) AS collected_at
		FROM knowledge_snapshots s
		WHERE s.project_id = $1
			AND s.branch = $2
			AND ($3 OR s.status <> 'failed')
		ORDER BY s.collected_at DESC, s.id DESC
		LIMIT 1`, projectID, branch, withFailed)
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func oneSnapshot(ctx context.Context, q querier, sql string, args ...any) (*knowledgetypes.Snapshot, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	r, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[snapshotRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	snap := r.snapshot()
	return &snap, nil
}

func (s *SnapshotRepository) Branches(ctx context.Context, projectID uuid.UUID) ([]knowledgetypes.Candidate, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT name, COALESCE(head_sha, '') AS head_sha, gone_at IS NOT NULL AS gone
		FROM branches
		WHERE project_id = $1
		ORDER BY name`, projectID)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (knowledgetypes.Candidate, error) {
		var c knowledgetypes.Candidate
		err := r.Scan(&c.Name, &c.HeadSHA, &c.Gone)
		return c, err
	})
	return out, postgres.Translate(err, constraints)
}

func (s *SnapshotRepository) LastAttempt(ctx context.Context, projectID uuid.UUID, branch string) (*knowledgetypes.Snapshot, error) {
	snap, err := lastSnapshot(ctx, s.db.From(ctx), projectID, branch, true)
	return snap, postgres.Translate(err, constraints)
}

func (s *SnapshotRepository) Latest(ctx context.Context, projectID uuid.UUID, branch string) (*knowledgetypes.Snapshot, error) {
	snap, err := lastSnapshot(ctx, s.db.From(ctx), projectID, branch, false)
	return snap, postgres.Translate(err, constraints)
}

func (s *SnapshotRepository) Known(ctx context.Context, projectID uuid.UUID, shas []string) (map[string]knowledgetypes.Known, error) {
	out := map[string]knowledgetypes.Known{}
	if len(shas) == 0 {
		return out, nil
	}
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT DISTINCT ON (f.git_blob_sha) f.git_blob_sha, b.sha256, b.content
		FROM knowledge_files f
		JOIN knowledge_snapshots s ON s.id = f.snapshot_id
		JOIN knowledge_blobs b ON b.sha256 = f.sha256
		WHERE s.project_id = $1
			AND f.git_blob_sha = ANY($2)`, projectID, shas)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	defer rows.Close()
	for rows.Next() {
		var git string
		var sum []byte
		var content string
		if err := rows.Scan(&git, &sum, &content); err != nil {
			return nil, postgres.Translate(err, constraints)
		}
		var k knowledgetypes.Known
		copy(k.SHA256[:], sum)
		k.Content = []byte(content)
		out[git] = k
	}
	return out, postgres.Translate(rows.Err(), constraints)
}

func (s *SnapshotRepository) resolve(ctx context.Context, projectID uuid.UUID, branch string, ref knowledgetypes.Ref) (*knowledgetypes.Snapshot, error) {
	if ref.Commit != nil {
		return oneSnapshot(ctx, s.db.From(ctx), `
			SELECT s.id, s.project_id, s.branch, s.commit_sha, s.status, s.error_code, s.files, s.bytes,
				s.skipped, s.truncated, rfc3339(s.collected_at) AS collected_at
			FROM knowledge_snapshots s
			WHERE s.project_id = $1
				AND s.branch = $2
				AND s.commit_sha = $3
				AND s.status <> 'failed'
			ORDER BY s.collected_at DESC, s.id DESC
			LIMIT 1`, projectID, branch, *ref.Commit)
	}
	return lastSnapshot(ctx, s.db.From(ctx), projectID, branch, false)
}

func scanInfo(r pgx.Row, extra ...any) (knowledgetypes.FileInfo, error) {
	var f knowledgetypes.FileInfo
	var kind string
	var skip *string
	var meta []byte
	if err := r.Scan(append([]any{&f.Path, &kind, &f.Bytes, &skip, &meta}, extra...)...); err != nil {
		return f, err
	}
	f.Kind = knowledgetypes.Kind(kind)
	if skip != nil {
		reason := knowledgetypes.SkipReason(*skip)
		f.Skip = &reason
	}
	if err := json.Unmarshal(meta, &f.Meta); err != nil {
		return f, err
	}
	return f, nil
}

func (s *SnapshotRepository) Files(ctx context.Context, projectID uuid.UUID, branch string, ref knowledgetypes.Ref) (knowledgetypes.Snapshot, []knowledgetypes.FileInfo, error) {
	snap, err := s.resolve(ctx, projectID, branch, ref)
	if err != nil {
		return knowledgetypes.Snapshot{}, nil, postgres.Translate(err, constraints)
	}
	if snap == nil {
		return knowledgetypes.Snapshot{}, nil, apperr.ErrNotFound
	}
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT path, kind, bytes, skip_reason, meta
		FROM knowledge_files
		WHERE snapshot_id = $1
		ORDER BY path COLLATE "C"`, snap.ID)
	if err != nil {
		return knowledgetypes.Snapshot{}, nil, postgres.Translate(err, constraints)
	}
	files, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (knowledgetypes.FileInfo, error) { return scanInfo(r) })
	return *snap, files, postgres.Translate(err, constraints)
}

func (s *SnapshotRepository) File(ctx context.Context, projectID uuid.UUID, branch string, ref knowledgetypes.Ref, path string) (knowledgetypes.Snapshot, knowledgetypes.FileContent, error) {
	snap, err := s.resolve(ctx, projectID, branch, ref)
	if err != nil {
		return knowledgetypes.Snapshot{}, knowledgetypes.FileContent{}, postgres.Translate(err, constraints)
	}
	if snap == nil {
		return knowledgetypes.Snapshot{}, knowledgetypes.FileContent{}, apperr.ErrNotFound
	}
	var f knowledgetypes.FileContent
	f.FileInfo, err = scanInfo(s.db.From(ctx).QueryRow(ctx, `
		SELECT f.path, f.kind, f.bytes, f.skip_reason, f.meta, b.content
		FROM knowledge_files f
		LEFT JOIN knowledge_blobs b ON b.sha256 = f.sha256
		WHERE f.snapshot_id = $1
			AND f.path = $2`, snap.ID, path), &f.Content)
	if errors.Is(err, pgx.ErrNoRows) {
		return knowledgetypes.Snapshot{}, knowledgetypes.FileContent{}, apperr.ErrNotFound
	}
	return *snap, f, postgres.Translate(err, constraints)
}

func (s *SnapshotRepository) PruneBlobs(ctx context.Context) (int64, error) {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM knowledge_blobs b
		WHERE NOT EXISTS (SELECT 1 FROM knowledge_files f WHERE f.sha256 = b.sha256)`)
	if err != nil {
		return 0, postgres.Translate(err, constraints)
	}
	return tag.RowsAffected(), nil
}

func (s *SnapshotRepository) ProjectsWithDocs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT DISTINCT project_id
		FROM knowledge_snapshots
		WHERE project_id = ANY($1)
			AND status <> 'failed'`, ids)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	found, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	for _, id := range found {
		out[id] = true
	}
	return out, nil
}
