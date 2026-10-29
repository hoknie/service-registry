package types

import "github.com/google/uuid"

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusDisabled  Status = "disabled"
)

func ParseStatus(s string) (Status, bool) {
	switch Status(s) {
	case StatusDraft, StatusPublished, StatusDisabled:
		return Status(s), true
	}
	return "", false
}

type Origin string

const (
	OriginUpload Origin = "upload"
	OriginGit    Origin = "git"
)

type Ref struct {
	ID   uuid.UUID
	Slug string
	Name string
}

type UserRef struct {
	ID   uuid.UUID
	Name string
}

type Scope struct {
	OrganizationID *uuid.UUID
}

func (s Scope) Global() bool { return s.OrganizationID == nil }

type Folder struct {
	ID             uuid.UUID
	ParentID       *uuid.UUID
	OrganizationID *uuid.UUID
	Organization   *Ref
	Slug           string
	Name           string
	Description    string
	Ancestors      []Ref
	Depth          int
	Skills         int64
	Folders        int64
	CanWrite       bool
	CreatedAt      string
	UpdatedAt      string
	Disabled       bool
	SourceID       *uuid.UUID
	DisabledBy     *Ref
}

func (f Folder) Managed() bool { return f.SourceID != nil }

func (f Folder) Off() bool { return f.Disabled || f.DisabledBy != nil }

type NewFolder struct {
	ID             uuid.UUID
	ParentID       *uuid.UUID
	OrganizationID *uuid.UUID
	Slug           string
	Name           string
	Description    string
}

type SourceRef struct {
	ID   uuid.UUID
	Kind string
	URL  string
}

type Skill struct {
	ID                uuid.UUID
	FolderID          *uuid.UUID
	OrganizationID    *uuid.UUID
	Organization      *Ref
	Path              string
	Slug              string
	Name              string
	Title             string
	Description       string
	About             string
	License           string
	Compatibility     string
	AllowedTools      []string
	Tags              []string
	Author            string
	ImagePath         *string
	HasUploadedImage  bool
	ImageType         *string
	ImageSHA256       []byte
	Status            Status
	Downloads         int64
	Source            *SourceRef
	SourcePath        *string
	CreatedBy         *UserRef
	CreatedAt         string
	UpdatedAt         string
	Versions          []Version
	Version           *Version
	VersionsAvailable int
	HasImage          bool
	CanWrite          bool
	DisabledBy        *Ref
}

func (s Skill) Active() bool { return s.Status == StatusPublished && s.DisabledBy == nil }

func (s Skill) Off() bool { return s.Status == StatusDisabled || s.DisabledBy != nil }

func (s Skill) Scope() Scope { return Scope{OrganizationID: s.OrganizationID} }

func (s Skill) Managed() bool { return s.Source != nil }

type Version struct {
	ID         uuid.UUID
	SkillID    uuid.UUID
	Version    string
	CreatedAt  string
	Origin     Origin
	Commit     *string
	Tag        *string
	CreatedBy  *UserRef
	Files      int
	Size       int64
	Digest     string
	Prerelease bool
	Yanked     bool
	Available  bool
	Manifest   Manifest
}

type FileKind string

const (
	KindMarkdown FileKind = "markdown"
	KindText     FileKind = "text"
	KindImage    FileKind = "image"
	KindBinary   FileKind = "binary"
)

type File struct {
	Path        string
	Size        int64
	SHA256      []byte
	Kind        FileKind
	ContentType string
	Title       *string
}

type FileContent struct {
	File
	Version string
	Content *string
}

type ListQuery struct {
	FolderID      *uuid.UUID
	TopLevel      bool
	Recursive     bool
	Organizations []uuid.UUID
	Global        bool
	Writable      []uuid.UUID
	WriteGlobal   bool
	OnlyOrg       *uuid.UUID
	OnlyGlobal    bool
	Query         string
	Tags          []string
	Status        *Status
	Active        *bool
	SetOrgs       []uuid.UUID
	IDs           []uuid.UUID
	Limit         int
	After         *Cursor
}

type Cursor struct {
	UpdatedAt string
	Name      string
	ID        uuid.UUID
}

type Visible struct {
	Organizations []uuid.UUID
	Global        bool
}

type TagCount struct {
	Tag   string
	Count int64
}

type Source struct {
	ID              uuid.UUID
	FolderID        uuid.UUID
	OrganizationID  *uuid.UUID
	Kind            string
	Hosting         *string
	URL             *string
	APIURL          *string
	Path            *string
	WorkingTree     bool
	SecretID        *uuid.UUID
	SecretName      *string
	Branch          string
	Roots           []string
	Layout          string
	GroupField      *string
	IntervalSecs    *int
	NewStatus       Status
	Versions        string
	TagPattern      string
	NextRunAt       *string
	Due             bool
	Running         bool
	LastCommit      *string
	LastTags        map[string]string
	LastRun         *Run
	ConfigChangedAt string
	CreatedAt       string
	UpdatedAt       string
	PausedReason    *string
	Requested       bool
	EffectiveSecs   int
}

const (
	PausedFolderDisabled = "folder_disabled"
	PausedSkillsDisabled = "skills_disabled"
	PausedPollingOff     = "polling_off"
)

type Run struct {
	StartedAt  string    `json:"started_at"`
	FinishedAt string    `json:"finished_at"`
	Status     string    `json:"status"`
	Commit     *string   `json:"commit"`
	Created    int       `json:"created"`
	Updated    int       `json:"updated"`
	Removed    int       `json:"removed"`
	Skipped    int       `json:"skipped"`
	Error      *string   `json:"error"`
	Problems   []Problem `json:"problems"`
}

type Problem struct {
	Code   string `json:"code"`
	Path   string `json:"path"`
	Detail string `json:"detail"`
}

const (
	RunSucceeded = "succeeded"
	RunUnchanged = "unchanged"
	RunFailed    = "failed"
	MaxProblems  = 20
)

func (r *Run) Problem(code, path, detail string) {
	if len(r.Problems) < MaxProblems {
		r.Problems = append(r.Problems, Problem{Code: code, Path: path, Detail: detail})
	}
}

func (s Source) State() string {
	switch {
	case s.Running:
		return "running"
	case s.NextRunAt != nil && s.Due && s.Requested:
		return "queued"
	case s.PausedReason != nil:
		return "paused"
	case s.NextRunAt != nil && s.Due:
		return "queued"
	case s.LastRun != nil && s.LastRun.Status == RunFailed:
		return "failed"
	}
	return "idle"
}
