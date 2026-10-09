package knowledge

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/knowledge"
)

type SnapshotStore struct{ pool *pgxpool.Pool }

func NewSnapshotStore(pool *pgxpool.Pool) *SnapshotStore { return &SnapshotStore{pool: pool} }

var (
	snapshotBranches = "SELECT name, COALESCE(head_sha, '') AS head_sha, gone_at IS NOT NULL AS gone FROM branches WHERE project_id = $1 ORDER BY name"
	snapshotLast     = "SELECT " + snapshotColumns + " FROM knowledge_snapshots s WHERE s.project_id = $1 AND s.branch = $2 " +
		"AND ($3 OR s.status <> 'failed') ORDER BY s.collected_at DESC, s.id DESC LIMIT 1"
	snapshotOfCommit = "SELECT " + snapshotColumns + " FROM knowledge_snapshots s WHERE s.project_id = $1 AND s.branch = $2 " +
		"AND s.commit_sha = $3 AND s.status <> 'failed' ORDER BY s.collected_at DESC, s.id DESC LIMIT 1"
	snapshotKnown = "SELECT DISTINCT ON (f.git_blob_sha) f.git_blob_sha, b.sha256, b.content FROM knowledge_files f " +
		"JOIN knowledge_snapshots s ON s.id = f.snapshot_id JOIN knowledge_blobs b ON b.sha256 = f.sha256 " +
		"WHERE s.project_id = $1 AND f.git_blob_sha = ANY($2)"
	snapshotTree  = "SELECT path, COALESCE(encode(sha256, 'hex'), 'skip:' || skip_reason) FROM knowledge_files WHERE snapshot_id = $1"
	snapshotTouch = "UPDATE knowledge_snapshots SET commit_sha = $2, status = $3, truncated = $4, collected_at = now() WHERE id = $1"
	blobInsert    = "INSERT INTO knowledge_blobs (sha256, content, bytes) " +
		"SELECT * FROM unnest($1::bytea[], $2::text[], $3::bigint[]) ON CONFLICT (sha256) DO NOTHING"
	blobLock       = "SELECT 1 FROM knowledge_blobs WHERE sha256 = ANY($1) FOR KEY SHARE"
	snapshotInsert = "INSERT INTO knowledge_snapshots (id, project_id, branch, commit_sha, status, error_code, files, bytes, skipped, truncated) " +
		"VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8, $9, $10)"
	fileInsert = "INSERT INTO knowledge_files (id, snapshot_id, path, git_blob_sha, sha256, bytes, skip_reason, kind, meta) " +
		"SELECT f.id, $1, f.path, f.git, f.sha, f.bytes, f.skip, f.kind, f.meta::jsonb " +
		"FROM unnest($2::uuid[], $3::text[], $4::text[], $5::bytea[], $6::bigint[], $7::text[], $8::text[], $9::text[]) " +
		"AS f(id, path, git, sha, bytes, skip, kind, meta)"
	snapshotDropFailed = "DELETE FROM knowledge_snapshots WHERE project_id = $1 AND branch = $2 AND status = 'failed' AND id <> $3"
	snapshotKeep       = "DELETE FROM knowledge_snapshots WHERE project_id = $1 AND branch = $2 AND status <> 'failed' AND id NOT IN (" +
		"SELECT id FROM knowledge_snapshots WHERE project_id = $1 AND branch = $2 AND status <> 'failed' " +
		"ORDER BY collected_at DESC, id DESC LIMIT $3)"
	fileList = "SELECT path, kind, bytes, skip_reason, meta FROM knowledge_files WHERE snapshot_id = $1 ORDER BY path COLLATE \"C\""
	fileOne  = "SELECT f.path, f.kind, f.bytes, f.skip_reason, f.meta, b.content FROM knowledge_files f " +
		"LEFT JOIN knowledge_blobs b ON b.sha256 = f.sha256 WHERE f.snapshot_id = $1 AND f.path = $2"
	blobPrune = "DELETE FROM knowledge_blobs b WHERE NOT EXISTS (SELECT 1 FROM knowledge_files f WHERE f.sha256 = b.sha256)"
)

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func oneSnapshot(ctx context.Context, q querier, sql string, args ...any) (*domain.Snapshot, error) {
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

func (s *SnapshotStore) Branches(ctx context.Context, projectID uuid.UUID) ([]domain.Candidate, error) {
	rows, err := s.pool.Query(ctx, snapshotBranches, projectID)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Candidate, error) {
		var c domain.Candidate
		err := r.Scan(&c.Name, &c.HeadSHA, &c.Gone)
		return c, err
	})
	return out, dbErr(err)
}

func (s *SnapshotStore) LastAttempt(ctx context.Context, projectID uuid.UUID, branch string) (*domain.Snapshot, error) {
	snap, err := oneSnapshot(ctx, s.pool, snapshotLast, projectID, branch, true)
	return snap, dbErr(err)
}

func (s *SnapshotStore) Latest(ctx context.Context, projectID uuid.UUID, branch string) (*domain.Snapshot, error) {
	snap, err := oneSnapshot(ctx, s.pool, snapshotLast, projectID, branch, false)
	return snap, dbErr(err)
}

func (s *SnapshotStore) Known(ctx context.Context, projectID uuid.UUID, shas []string) (map[string]domain.Known, error) {
	out := map[string]domain.Known{}
	if len(shas) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, snapshotKnown, projectID, shas)
	if err != nil {
		return nil, dbErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		var git string
		var sum []byte
		var content string
		if err := rows.Scan(&git, &sum, &content); err != nil {
			return nil, dbErr(err)
		}
		var k domain.Known
		copy(k.SHA256[:], sum)
		k.Content = []byte(content)
		out[git] = k
	}
	return out, dbErr(rows.Err())
}

func (s *SnapshotStore) Save(ctx context.Context, n domain.NewSnapshot, keep uint32) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if n.Status == domain.StatusFailed {
			if _, err := tx.Exec(ctx, snapshotInsert, n.ID, n.ProjectID, n.Branch, n.Commit, string(n.Status), n.ErrorCode, 0, 0, 0, false); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, snapshotDropFailed, n.ProjectID, n.Branch, n.ID)
			return err
		}
		latest, err := oneSnapshot(ctx, tx, snapshotLast, n.ProjectID, n.Branch, false)
		if err != nil {
			return err
		}
		if latest != nil {
			same, err := sameTree(ctx, tx, latest.ID, n.Files)
			if err != nil {
				return err
			}
			if same {
				if _, err := tx.Exec(ctx, snapshotTouch, latest.ID, n.Commit, string(n.Status), n.Truncated); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, snapshotDropFailed, n.ProjectID, n.Branch, latest.ID)
				return err
			}
		}
		if err := insertSnapshot(ctx, tx, n); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, snapshotDropFailed, n.ProjectID, n.Branch, n.ID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, snapshotKeep, n.ProjectID, n.Branch, int64(keep)+1)
		return err
	})
	return dbErr(err)
}

func sameTree(ctx context.Context, tx pgx.Tx, snapshotID uuid.UUID, files []domain.File) (bool, error) {
	stored := map[string]string{}
	rows, err := tx.Query(ctx, snapshotTree, snapshotID)
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
		if stored[f.Path] != want {
			return false, nil
		}
	}
	return true, nil
}

func insertSnapshot(ctx context.Context, tx pgx.Tx, n domain.NewSnapshot) error {
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
		if _, err := tx.Exec(ctx, blobInsert, sums, contents, sizes); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, blobLock, sums); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, snapshotInsert, n.ID, n.ProjectID, n.Branch, n.Commit, string(n.Status), "", stored, total, skipped, n.Truncated); err != nil {
		return err
	}
	if len(n.Files) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, fileInsert, n.ID, ids, paths, gits, fileSums, bytes, skips, kinds, metas)
	return err
}

func (s *SnapshotStore) resolve(ctx context.Context, projectID uuid.UUID, branch string, ref domain.Ref) (*domain.Snapshot, error) {
	if ref.Commit != nil {
		return oneSnapshot(ctx, s.pool, snapshotOfCommit, projectID, branch, *ref.Commit)
	}
	return oneSnapshot(ctx, s.pool, snapshotLast, projectID, branch, false)
}

func scanInfo(r pgx.Row, extra ...any) (domain.FileInfo, error) {
	var f domain.FileInfo
	var kind string
	var skip *string
	var meta []byte
	if err := r.Scan(append([]any{&f.Path, &kind, &f.Bytes, &skip, &meta}, extra...)...); err != nil {
		return f, err
	}
	f.Kind = domain.Kind(kind)
	if skip != nil {
		reason := domain.SkipReason(*skip)
		f.Skip = &reason
	}
	if err := json.Unmarshal(meta, &f.Meta); err != nil {
		return f, err
	}
	return f, nil
}

func (s *SnapshotStore) Files(ctx context.Context, projectID uuid.UUID, branch string, ref domain.Ref) (domain.Snapshot, []domain.FileInfo, error) {
	snap, err := s.resolve(ctx, projectID, branch, ref)
	if err != nil {
		return domain.Snapshot{}, nil, dbErr(err)
	}
	if snap == nil {
		return domain.Snapshot{}, nil, domain.ErrNotFound
	}
	rows, err := s.pool.Query(ctx, fileList, snap.ID)
	if err != nil {
		return domain.Snapshot{}, nil, dbErr(err)
	}
	files, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.FileInfo, error) { return scanInfo(r) })
	return *snap, files, dbErr(err)
}

func (s *SnapshotStore) File(ctx context.Context, projectID uuid.UUID, branch string, ref domain.Ref, path string) (domain.Snapshot, domain.FileContent, error) {
	snap, err := s.resolve(ctx, projectID, branch, ref)
	if err != nil {
		return domain.Snapshot{}, domain.FileContent{}, dbErr(err)
	}
	if snap == nil {
		return domain.Snapshot{}, domain.FileContent{}, domain.ErrNotFound
	}
	var f domain.FileContent
	f.FileInfo, err = scanInfo(s.pool.QueryRow(ctx, fileOne, snap.ID, path), &f.Content)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Snapshot{}, domain.FileContent{}, domain.ErrNotFound
	}
	return *snap, f, dbErr(err)
}

func (s *SnapshotStore) PruneBlobs(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, blobPrune)
	if err != nil {
		return 0, dbErr(err)
	}
	return tag.RowsAffected(), nil
}
