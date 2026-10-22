package repositories

import (
	"github.com/google/uuid"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/postgres"
)

type snapshotRow struct {
	ID          uuid.UUID `db:"id"`
	ProjectID   uuid.UUID `db:"project_id"`
	Branch      string    `db:"branch"`
	CommitSHA   string    `db:"commit_sha"`
	Status      string    `db:"status"`
	ErrorCode   *string   `db:"error_code"`
	Files       int32     `db:"files"`
	Bytes       int64     `db:"bytes"`
	Skipped     int32     `db:"skipped"`
	Truncated   bool      `db:"truncated"`
	CollectedAt string    `db:"collected_at"`
}

func (r snapshotRow) snapshot() knowledgetypes.Snapshot {
	return knowledgetypes.Snapshot{ID: r.ID, ProjectID: r.ProjectID, Branch: r.Branch, Commit: r.CommitSHA, Status: knowledgetypes.Status(r.Status),
		ErrorCode: r.ErrorCode, Files: int(r.Files), Bytes: r.Bytes, Skipped: int(r.Skipped), Truncated: r.Truncated, CollectedAt: r.CollectedAt}
}

func Err(err error) error { return postgres.Translate(err, constraints) }

type knownRow struct {
	Git     string
	SHA256  []byte
	Content string
}

type blobRow struct {
	SHA256  []byte
	Content string
}

type chunkRow struct {
	SHA256 []byte
	Chunk  knowledgetypes.StoredChunk
}

type projectNameRow struct {
	ID   uuid.UUID
	Name string
	Path string
}

type pathKeyRow struct {
	Path string
	Key  string
}

type countsRow struct {
	Pending int
	Indexed int
}
