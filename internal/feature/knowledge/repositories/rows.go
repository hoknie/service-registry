package repositories

import (
	"encoding/json"

	"github.com/google/uuid"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/postgres"
)

type snapshotRow struct {
	ID          uuid.UUID `db:"id"`
	ProjectID   uuid.UUID `db:"project_id"`
	Branch      string    `db:"branch"`
	RefKind     string    `db:"ref_kind"`
	CommitSHA   string    `db:"commit_sha"`
	Status      string    `db:"status"`
	ErrorCode   *string   `db:"error_code"`
	Files       int32     `db:"files"`
	Bytes       int64     `db:"bytes"`
	Skipped     int32     `db:"skipped"`
	Truncated   bool      `db:"truncated"`
	CollectedAt string    `db:"collected_at"`
	Limited     int32     `db:"limited"`
	TooLarge    int32     `db:"too_large"`
}

func (r snapshotRow) snapshot() knowledgetypes.Snapshot {
	return knowledgetypes.Snapshot{ID: r.ID, ProjectID: r.ProjectID, Branch: r.Branch, RefKind: knowledgetypes.RefKind(r.RefKind), Commit: r.CommitSHA, Status: knowledgetypes.Status(r.Status),
		ErrorCode: r.ErrorCode, Files: int(r.Files), Bytes: r.Bytes, Skipped: int(r.Skipped), Limited: int(r.Limited), TooLarge: int(r.TooLarge),
		Truncated: r.Truncated, CollectedAt: r.CollectedAt}
}

func Err(err error) error { return postgres.Translate(err, constraints) }

type contentRow struct {
	Path    string
	Content string
}

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

type indexFileRow struct {
	Branch    string `db:"branch"`
	CommitSHA string `db:"commit_sha"`
	Path      string `db:"path"`
	Kind      string `db:"kind"`
	SHA256    []byte `db:"sha256"`
}

func (r indexFileRow) file() knowledgetypes.IndexFile {
	return knowledgetypes.IndexFile{Branch: r.Branch, Commit: r.CommitSHA, Path: r.Path, Kind: knowledgetypes.Kind(r.Kind), SHA256: r.SHA256}
}

type hitRow struct {
	ProjectID   uuid.UUID `db:"project_id"`
	ProjectName string    `db:"name"`
	ProjectPath string    `db:"project_path"`
	Branch      string    `db:"branch"`
	CommitSHA   string    `db:"commit_sha"`
	Path        string    `db:"path"`
	Kind        string    `db:"kind"`
	Fragment    string    `db:"fragment"`
}

func (r hitRow) hit() knowledgetypes.Hit {
	return knowledgetypes.Hit{ProjectID: r.ProjectID, ProjectName: r.ProjectName, ProjectPath: r.ProjectPath, Branch: r.Branch,
		Commit: r.CommitSHA, Path: r.Path, Kind: knowledgetypes.Kind(r.Kind), Snippet: knowledgetypes.Snippet(r.Fragment)}
}

type publicHitRow struct {
	hitRow
	Head *string `db:"head"`
}

func (r publicHitRow) hit() knowledgetypes.Hit {
	h := r.hitRow.hit()
	if r.Head != nil {
		h.Title = knowledgetypes.MarkdownTitle(*r.Head)
	}
	return h
}

type scanArgs struct {
	branches, index, notes, counts []byte
	code, detail, source           *string
}

func scanArgsOf(scan knowledgetypes.Scan) (scanArgs, error) {
	var a scanArgs
	var err error
	if a.branches, err = json.Marshal(scan.Branches); err != nil {
		return a, err
	}
	if scan.Index != nil {
		if a.index, err = json.Marshal(scan.Index); err != nil {
			return a, err
		}
	}
	if a.notes, err = json.Marshal(scan.Notes); err != nil {
		return a, err
	}
	if a.counts, err = json.Marshal(scan.Counts); err != nil {
		return a, err
	}
	if scan.Error != nil {
		a.code, a.detail = &scan.Error.Code, &scan.Error.Detail
	}
	if scan.Source != "" {
		a.source = &scan.Source
	}
	return a, nil
}

type refRow struct {
	Pattern string `json:"pattern"`
	Kind    string `json:"kind"`
	Release bool   `json:"release"`
}

func refsOf(rows *[]refRow) *[]knowledgetypes.RefPattern {
	if rows == nil {
		return nil
	}
	out := make([]knowledgetypes.RefPattern, len(*rows))
	for i, r := range *rows {
		out[i] = knowledgetypes.RefPattern{Pattern: r.Pattern, Kind: knowledgetypes.RefKind(r.Kind), Release: r.Release}
	}
	return &out
}

func refRows(refs *[]knowledgetypes.RefPattern) *[]refRow {
	if refs == nil {
		return nil
	}
	out := make([]refRow, len(*refs))
	for i, r := range *refs {
		out[i] = refRow{Pattern: r.Pattern, Kind: string(r.Kind), Release: r.Release}
	}
	return &out
}
