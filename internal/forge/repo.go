package forge

import (
	"time"

	"github.com/google/uuid"
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
}

type Readme struct {
	Markdown      string
	Truncated     bool
	WebURL        string
	DefaultBranch *string
	Kind          Kind
}
