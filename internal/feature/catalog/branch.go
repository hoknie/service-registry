package catalog

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type BranchSource string

const (
	SourceForge      BranchSource = "forge"
	SourceRepository BranchSource = "repository"
	SourceIngest     BranchSource = "ingest"
	SourceCluster    BranchSource = "cluster"
	SourceManual     BranchSource = "manual"
)

var BranchSources = []BranchSource{SourceForge, SourceRepository, SourceIngest, SourceCluster, SourceManual}

type Branch struct {
	ProjectID      uuid.UUID
	Name           string
	HeadSHA        *string
	IsDefault      bool
	Protected      *bool
	Sources        []BranchSource
	Pinned         bool
	Stale          bool
	FirstSeenAt    string
	LastActivityAt string
	GoneAt         *string
}

type BranchState string

const (
	BranchesActive BranchState = "active"
	BranchesStale  BranchState = "stale"
	BranchesGone   BranchState = "gone"
	BranchesAll    BranchState = "all"
)

type BranchQuery struct {
	Prefix *string
	State  *string
	Limit  *int64
	Offset *int64
}

type BranchFilter struct {
	Prefix string
	State  BranchState
	Limit  uint32
	Offset uint64
}

type ForgeBranch struct {
	Name        string
	HeadSHA     string
	Protected   *bool
	CommittedAt *time.Time
}

type IngestBranch struct {
	ProjectID  uuid.UUID
	Name       string
	CommitSHA  *string
	OccurredAt time.Time
}

func BranchName(raw string) (string, error) {
	name := strings.TrimPrefix(strings.TrimSpace(raw), "refs/heads/")
	b, err := ValidateBranch(name)
	if err != nil {
		return "", err
	}
	if b == nil {
		return "", InvalidBranch
	}
	return *b, nil
}

func ValidateBranchQuery(q BranchQuery) (BranchFilter, error) {
	f := BranchFilter{State: BranchesActive, Limit: 50}
	if q.Prefix != nil {
		f.Prefix = *q.Prefix
	}
	if q.State != nil {
		switch s := BranchState(*q.State); s {
		case BranchesActive, BranchesStale, BranchesGone, BranchesAll:
			f.State = s
		default:
			return BranchFilter{}, InvalidBranchState
		}
	}
	if q.Limit != nil {
		if *q.Limit < 1 || *q.Limit > 200 {
			return BranchFilter{}, InvalidBranchPage
		}
		f.Limit = uint32(*q.Limit)
	}
	if q.Offset != nil {
		if *q.Offset < 0 {
			return BranchFilter{}, InvalidBranchPage
		}
		f.Offset = uint64(*q.Offset)
	}
	return f, nil
}
