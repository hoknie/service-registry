package repositories

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

func (s *SnapshotRepository) Save(ctx context.Context, n knowledgetypes.NewSnapshot, keep uint32) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		if n.Status == knowledgetypes.StatusFailed {
			if err := addSnapshotRow(ctx, tx, n, n.ErrorCode, 0, 0, 0, false); err != nil {
				return err
			}
			return dropFailedSnapshots(ctx, tx, n.ProjectID, n.Branch, n.ID)
		}
		latest, err := lastSnapshot(ctx, tx, n.ProjectID, n.Branch, false)
		if err != nil {
			return err
		}
		if latest != nil {
			same, err := sameTree(ctx, tx, latest.ID, n.Files)
			if err != nil {
				return err
			}
			if same {
				if _, err := tx.Exec(ctx, `
					UPDATE knowledge_snapshots
					SET commit_sha = $2, status = $3, truncated = $4, collected_at = now()
					WHERE id = $1`, latest.ID, n.Commit, string(n.Status), n.Truncated); err != nil {
					return err
				}
				return dropFailedSnapshots(ctx, tx, n.ProjectID, n.Branch, latest.ID)
			}
		}
		if err := insertSnapshot(ctx, tx, n); err != nil {
			return err
		}
		if err := dropFailedSnapshots(ctx, tx, n.ProjectID, n.Branch, n.ID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			DELETE FROM knowledge_snapshots
			WHERE project_id = $1
				AND branch = $2
				AND status <> 'failed'
				AND id NOT IN (
					SELECT id
					FROM knowledge_snapshots
					WHERE project_id = $1
						AND branch = $2
						AND status <> 'failed'
					ORDER BY collected_at DESC, id DESC
					LIMIT $3
				)`, n.ProjectID, n.Branch, int64(keep)+1)
		return err
	})
	return postgres.Translate(err, constraints)
}

func sameTree(ctx context.Context, tx pgtx.Querier, snapshotID uuid.UUID, files []knowledgetypes.File) (bool, error) {
	stored := map[string]string{}
	rows, err := tx.Query(ctx, `
		SELECT path, COALESCE(encode(sha256, 'hex'), 'skip:' || skip_reason) || ':' || COALESCE(meta ->> 'git', '')
		FROM knowledge_files
		WHERE snapshot_id = $1`, snapshotID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var path, key string
		if err := rows.Scan(&path, &key); err != nil {
			return false, err
		}
		stored[path] = key
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if len(stored) != len(files) {
		return false, nil
	}
	for _, f := range files {
		want := "skip:" + string(f.Skip)
		if f.Skip == "" {
			want = hex.EncodeToString(f.SHA256[:])
		}
		if stored[f.Path] != want+":"+string(f.Meta.Git) {
			return false, nil
		}
	}
	return true, nil
}

func insertSnapshot(ctx context.Context, tx pgtx.Querier, n knowledgetypes.NewSnapshot) error {
	var sums [][]byte
	var contents []string
	var sizes []int64
	seen := map[[32]byte]bool{}
	var stored, skipped int
	var total int64
	ids := make([]uuid.UUID, len(n.Files))
	paths := make([]string, len(n.Files))
	gits := make([]string, len(n.Files))
	fileSums := make([][]byte, len(n.Files))
	bytes := make([]int64, len(n.Files))
	skips := make([]*string, len(n.Files))
	kinds := make([]string, len(n.Files))
	metas := make([]string, len(n.Files))
	for i, f := range n.Files {
		ids[i], paths[i], gits[i], bytes[i], kinds[i] = n.FileIDs[i], f.Path, f.GitBlobSHA, f.Bytes, string(f.Kind)
		meta, err := json.Marshal(f.Meta)
		if err != nil {
			return err
		}
		metas[i] = string(meta)
		if f.Skip != "" {
			reason := string(f.Skip)
			skips[i] = &reason
			skipped++
			continue
		}
		sum := f.SHA256
		fileSums[i] = sum[:]
		stored++
		total += f.Bytes
		if !seen[sum] {
			seen[sum] = true
			sums = append(sums, sum[:])
			contents = append(contents, string(f.Content))
			sizes = append(sizes, f.Bytes)
		}
	}
	if len(sums) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO knowledge_blobs (sha256, content, bytes)
			SELECT *
			FROM unnest($1::bytea[], $2::text[], $3::bigint[])
			ON CONFLICT (sha256)
			DO NOTHING`, sums, contents, sizes); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			SELECT 1
			FROM knowledge_blobs
			WHERE sha256 = ANY($1) FOR KEY SHARE`, sums); err != nil {
			return err
		}
	}
	if err := addSnapshotRow(ctx, tx, n, "", stored, total, skipped, n.Truncated); err != nil {
		return err
	}
	if len(n.Files) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO knowledge_files (id, snapshot_id, path, git_blob_sha, sha256, bytes, skip_reason, kind,
			meta)
		SELECT f.id, $1, f.path, f.git, f.sha, f.bytes, f.skip, f.kind, f.meta::jsonb
		FROM unnest($2::uuid[], $3::text[], $4::text[], $5::bytea[], $6::bigint[], $7::text[], $8::text[],
			$9::text[]) AS f(id, path, git, sha, bytes, skip, kind, meta)`, n.ID, ids, paths, gits, fileSums, bytes, skips, kinds, metas)
	return err
}

func addSnapshotRow(ctx context.Context, tx pgtx.Querier, n knowledgetypes.NewSnapshot, errorCode string, files int, bytes int64, skipped int, truncated bool) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO knowledge_snapshots (id, project_id, branch, commit_sha, status, error_code, files,
			bytes, skipped, truncated)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8, $9, $10)`,
		n.ID, n.ProjectID, n.Branch, n.Commit, string(n.Status), errorCode, files, bytes, skipped, truncated)
	return err
}

func dropFailedSnapshots(ctx context.Context, tx pgtx.Querier, projectID uuid.UUID, branch string, keep uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM knowledge_snapshots
		WHERE project_id = $1
			AND branch = $2
			AND status = 'failed'
			AND id <> $3`, projectID, branch, keep)
	return err
}
