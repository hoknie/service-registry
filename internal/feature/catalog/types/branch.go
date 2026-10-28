package types

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type BranchSource string

const (
	SourceRepoSync   BranchSource = "repo_sync"
	SourceRepository BranchSource = "repository"
	SourceIngest     BranchSource = "ingest"
	SourceCluster    BranchSource = "cluster"
	SourceManual     BranchSource = "manual"
)

var BranchSources = []BranchSource{SourceRepoSync, SourceRepository, SourceIngest, SourceCluster, SourceManual}

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

const MaxBranchLen = 255

func BranchFits(name string) bool {
	return name != "" && utf8.RuneCountInString(name) <= MaxBranchLen
}

type RepoSyncBranch struct {
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

var hexSHA = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

func BranchSHA(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.ToLower(*s)
	if !hexSHA.MatchString(v) {
		return nil
	}
	return &v
}

type BranchHead struct {
	ID   uuid.UUID
	Name string
	Head *string
}

func HeadsMoved(known map[string]string, branches []RepoSyncBranch) bool {
	for _, b := range branches {
		if head := BranchSHA(&b.HeadSHA); head != nil && known[b.Name] != *head {
			return true
		}
	}
	return false
}
