package types

import (
	"context"
	"time"

	"github.com/google/uuid"
)

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
	Limited     int
	TooLarge    int
	Truncated   bool
	CollectedAt string
	Limits      []LimitSetting
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

type SnapshotFiles struct {
	Snapshot Snapshot
	Files    []FileInfo
	Contents func(ctx context.Context, paths []string) (map[string]string, error)
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
	TriggeredAt   *time.Time
	Editor        bool
}

type Release struct {
	AfterSecs uint32
	OnEvent   bool
	Retry     bool
	Seen      *time.Time
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
	Nodes      []uuid.UUID
	Superadmin bool
	Admin      bool
}
