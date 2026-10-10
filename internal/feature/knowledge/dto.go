package knowledge

import "github.com/google/uuid"

type Snapshot struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Branch      string
	Commit      string
	Status      Status
	ErrorCode   *string
	Files       int
	Bytes       int64
	Skipped     int
	Truncated   bool
	CollectedAt string
}

type NewSnapshot struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	Branch    string
	Commit    string
	Status    Status
	ErrorCode string
	Truncated bool
	Files     []File
	FileIDs   []uuid.UUID
}

type FileInfo struct {
	Path  string
	Kind  Kind
	Bytes int64
	Skip  *SkipReason
	Meta  Meta
}

type FileContent struct {
	FileInfo
	Content *string
}

type Ref struct {
	Branch *string
	Commit *string
}

type Known struct {
	SHA256  [32]byte
	Content []byte
}

type BranchState struct {
	Name     string
	HeadSHA  *string
	Pending  bool
	Last     *Snapshot
	Snapshot *Snapshot
}

type Candidate struct {
	Name    string
	HeadSHA string
	Gone    bool
}

type Project struct {
	ID            uuid.UUID
	DefaultBranch string
	Synced        bool
	ConnectionID  *uuid.UUID
	Settings      Settings
	Force         bool
}

type Hit struct {
	ProjectID   uuid.UUID
	ProjectPath string
	ProjectName string
	Branch      string
	Commit      string
	Path        string
	Kind        Kind
	Snippet     []Segment
}

type Viewer struct {
	UserID     uuid.UUID
	Superadmin bool
}
