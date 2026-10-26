package repositories

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/postgres"
)

func (s *SnapshotRepository) TreeKeys(ctx context.Context, snapshotID uuid.UUID) (map[string]string, error) {
	rows, err := postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowToStructByPos[pathKeyRow], `
		SELECT path, COALESCE(encode(sha256, 'hex'), 'skip:' || skip_reason) || ':' || COALESCE(meta ->> 'git', '')
		FROM knowledge_files
		WHERE snapshot_id = $1`, snapshotID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Path] = r.Key
	}
	return out, nil
}

func (s *SnapshotRepository) Refresh(ctx context.Context, snapshotID uuid.UUID, n knowledgetypes.NewSnapshot) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE knowledge_snapshots
		SET commit_sha = $2, status = $3, truncated = $4, collected_at = now()
		WHERE id = $1`, snapshotID, n.CommitSHA, string(n.Status), n.Truncated)
	return err
}

func (s *SnapshotRepository) AddBlobs(ctx context.Context, files []knowledgetypes.File) error {
	var sums [][]byte
	var contents []string
	var sizes []int64
	seen := map[[32]byte]bool{}
	for _, f := range files {
		if f.Skip != "" || seen[f.SHA256] {
			continue
		}
		seen[f.SHA256] = true
		sums = append(sums, f.SHA256[:])
		contents = append(contents, string(f.Content))
		sizes = append(sizes, f.Bytes)
	}
	if len(sums) == 0 {
		return nil
	}
	tx := s.db.From(ctx)
	if _, err := postgres.Exec(ctx, tx, constraints, `
		INSERT INTO knowledge_blobs (sha256, content, bytes)
		SELECT *
		FROM unnest($1::bytea[], $2::text[], $3::bigint[])
		ON CONFLICT (sha256)
		DO NOTHING`, sums, contents, sizes); err != nil {
		return err
	}
	_, err := postgres.Exec(ctx, tx, constraints, `
		SELECT 1
		FROM knowledge_blobs
		WHERE sha256 = ANY($1) FOR KEY SHARE`, sums)
	return err
}

func (s *SnapshotRepository) Add(ctx context.Context, n knowledgetypes.NewSnapshot, t knowledgetypes.SnapshotTotals) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO knowledge_snapshots (id, project_id, branch, ref_kind, commit_sha, status, error_code, files,
			bytes, skipped, truncated)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9, $10, $11)`,
		n.ID, n.ProjectID, n.Branch, string(n.RefKind), n.CommitSHA, string(n.Status), n.ErrorCode, t.Stored, t.Bytes, t.Skipped, n.Truncated)
	return err
}

func (s *SnapshotRepository) AddFiles(ctx context.Context, n knowledgetypes.NewSnapshot) error {
	if len(n.Files) == 0 {
		return nil
	}
	paths := make([]string, len(n.Files))
	gits := make([]string, len(n.Files))
	sums := make([][]byte, len(n.Files))
	bytes := make([]int64, len(n.Files))
	skips := make([]*string, len(n.Files))
	kinds := make([]string, len(n.Files))
	metas := make([]string, len(n.Files))
	for i, f := range n.Files {
		paths[i], gits[i], bytes[i], kinds[i] = f.Path, f.GitBlobSHA, f.Bytes, string(f.Kind)
		meta, err := json.Marshal(f.Meta)
		if err != nil {
			return err
		}
		metas[i] = string(meta)
		if f.Skip != "" {
			skips[i] = new(string(f.Skip))
			continue
		}
		sum := f.SHA256
		sums[i] = sum[:]
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO knowledge_files (id, snapshot_id, path, git_blob_sha, sha256, bytes, skip_reason, kind,
			meta)
		SELECT f.id, $1, f.path, f.git, f.sha, f.bytes, f.skip, f.kind, f.meta::jsonb
		FROM unnest($2::uuid[], $3::text[], $4::text[], $5::bytea[], $6::bigint[], $7::text[], $8::text[],
			$9::text[]) AS f(id, path, git, sha, bytes, skip, kind, meta)`, n.ID, n.FileIDs, paths, gits, sums, bytes, skips, kinds, metas)
	return err
}

func (s *SnapshotRepository) DropFailed(ctx context.Context, projectID uuid.UUID, kind knowledgetypes.RefKind, name string, keep uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM knowledge_snapshots
		WHERE project_id = $1
			AND ref_kind = $2
			AND branch = $3
			AND status = 'failed'
			AND id <> $4`, projectID, string(kind), name, keep)
	return err
}

func (s *SnapshotRepository) Trim(ctx context.Context, projectID uuid.UUID, kind knowledgetypes.RefKind, name string, keep int64) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM knowledge_snapshots
		WHERE project_id = $1
			AND ref_kind = $2
			AND branch = $3
			AND status <> 'failed'
			AND id NOT IN (
				SELECT id
				FROM knowledge_snapshots
				WHERE project_id = $1
					AND ref_kind = $2
					AND branch = $3
					AND status <> 'failed'
				ORDER BY collected_at DESC, id DESC
				LIMIT $4
			)`, projectID, string(kind), name, keep)
	return err
}

func (s *SnapshotRepository) DeleteTags(ctx context.Context, projectID uuid.UUID, names []string) error {
	if len(names) == 0 {
		return nil
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM knowledge_snapshots
		WHERE project_id = $1
			AND ref_kind = 'tag'
			AND branch = ANY($2)`, projectID, names)
	return err
}
