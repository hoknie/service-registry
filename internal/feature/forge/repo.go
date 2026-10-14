package forge

import (
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog"
)

type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityInternal Visibility = "internal"
	VisibilityPrivate  Visibility = "private"
)

type RemoteRepo struct {
	ExternalID    string
	FullPath      string
	Name          string
	Description   string
	Topics        []string
	WebURL        string
	DefaultBranch string
	Archived      bool
	Fork          bool
	Visibility    Visibility
	Stars         int
	License       string
	ReadmeHint    string
	PushedAt      *time.Time
	UpdatedAt     *time.Time
}

const (
	MaxPathLen    = 1024
	MaxURLLen     = 2048
	MaxLicenseLen = 255
)

func (r RemoteRepo) TooLong() bool {
	return utf8.RuneCountInString(r.FullPath) > MaxPathLen || utf8.RuneCountInString(r.WebURL) > MaxURLLen
}

func (r RemoteRepo) Bounded() RemoteRepo {
	if !catalog.BranchFits(r.DefaultBranch) {
		r.DefaultBranch = ""
	}
	if utf8.RuneCountInString(r.License) > MaxLicenseLen {
		r.License = ""
	}
	return r
}

const CodeValueTooLong = "forge.value_too_long"

func ProblemPath(path string) string {
	if utf8.RuneCountInString(path) <= MaxPathLen {
		return path
	}
	return string([]rune(path)[:MaxPathLen])
}

func BoundedLicense(l *string) *string {
	if l == nil || *l == "" || utf8.RuneCountInString(*l) > MaxLicenseLen {
		return nil
	}
	return l
}

func (r RemoteRepo) ChangedAt() *time.Time {
	switch {
	case r.PushedAt == nil:
		return r.UpdatedAt
	case r.UpdatedAt == nil || r.PushedAt.After(*r.UpdatedAt):
		return r.PushedAt
	}
	return r.UpdatedAt
}

type Details struct {
	Readme    *string
	Truncated bool
	Languages map[string]float64
	License   *string
}

const ReadmeMaxBytes = 512 << 10

type Link struct {
	ProjectID       uuid.UUID
	ExternalID      string
	FullPath        string
	Orphaned        bool
	SourceUpdatedAt *time.Time
}

type Repository struct {
	ConnectionID  uuid.UUID
	ExternalID    string
	FullPath      string
	WebURL        string
	Description   string
	Topics        []string
	Languages     map[string]float64
	DefaultBranch *string
	Archived      bool
	Visibility    Visibility
	Stars         int
	License       *string
	PushedAt      *string
	SyncedAt      string
	OrphanedAt    *string
	HasReadme     bool
	Kind          Kind
	Origin        string
}

const (
	OriginForge  = "forge"
	OriginSource = "source"
)

type Readme struct {
	Markdown      string
	Truncated     bool
	WebURL        string
	DefaultBranch *string
	Kind          Kind
}
