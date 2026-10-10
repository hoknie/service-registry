package knowledge

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ScanKind string

const (
	CollectScan ScanKind = "collect"
	IndexScan   ScanKind = "index"
)

type ScanTrigger string

const (
	TriggerSchedule ScanTrigger = "schedule"
	TriggerManual   ScanTrigger = "manual"
)

type ScanStatus string

const (
	ScanOK        ScanStatus = "ok"
	ScanUnchanged ScanStatus = "unchanged"
	ScanWarning   ScanStatus = "warning"
	ScanFailed    ScanStatus = "failed"
)

type BranchResult string

const (
	BranchCollected BranchResult = "collected"
	BranchUnchanged BranchResult = "unchanged"
	BranchFailed    BranchResult = "failed"
)

const (
	WarnNoFilesMatched = "collect.no_files_matched"
	WarnFilesSkipped   = "collect.files_skipped"
	WarnTruncated      = "collect.truncated"
	WarnNoBranches     = "collect.no_branches"
	WarnNoSource       = "collect.no_source"
)

const ScanDetailLimit = 1000

type ScanBranch struct {
	Name        string         `json:"name"`
	Commit      string         `json:"commit"`
	Result      BranchResult   `json:"result"`
	Files       int            `json:"files"`
	Skipped     map[string]int `json:"skipped"`
	Truncated   bool           `json:"truncated"`
	WorkingTree bool           `json:"working_tree,omitempty"`
	Error       string         `json:"error,omitempty"`
}

type ScanIndex struct {
	EmbeddedFiles  int    `json:"embedded_files"`
	EmbeddedChunks int    `json:"embedded_chunks"`
	Documents      int    `json:"documents"`
	Synced         bool   `json:"synced"`
	Engine         string `json:"engine"`
	Model          string `json:"model"`
}

type ScanError struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

type Scan struct {
	ID         uuid.UUID
	ProjectID  uuid.UUID
	Kind       ScanKind
	Trigger    ScanTrigger
	Source     string
	Status     ScanStatus
	StartedAt  time.Time
	FinishedAt time.Time
	Branches   []ScanBranch
	Index      *ScanIndex
	Error      *ScanError
	Warnings   []string
}

type ScanItem struct {
	ID            uuid.UUID
	ProjectID     uuid.UUID
	ProjectPath   string
	ProjectName   string
	Kind          ScanKind
	Trigger       ScanTrigger
	Source        *string
	Status        ScanStatus
	StartedAt     string
	LastStartedAt string
	FinishedAt    string
	DurationMS    *int64
	Repeats       int
	Branches      []ScanBranch
	Index         *ScanIndex
	Error         *ScanError
	Warnings      []string
}

type ScanTail struct {
	ID         uuid.UUID
	Status     ScanStatus
	Trigger    ScanTrigger
	Branches   []ScanBranch
	FinishedAt time.Time
}

func (s Scan) Extends(last ScanTail, maxGap time.Duration) bool {
	return s.Status == ScanUnchanged && last.Status == ScanUnchanged && s.Trigger == last.Trigger &&
		SameBranches(s.Branches, last.Branches) && s.StartedAt.Sub(last.FinishedAt) <= maxGap
}

func (s Scan) DurationMS() int64 {
	return max(s.FinishedAt.Sub(s.StartedAt).Milliseconds(), 0)
}

func SameBranches(a, b []ScanBranch) bool {
	key := func(list []ScanBranch) []string {
		out := make([]string, len(list))
		for i, x := range list {
			out[i] = x.Name + "\x00" + x.Commit
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(key(a), key(b))
}

type ScanQuery struct {
	Project string
	Kind    string
	Status  string
	Trigger string
}

type ScanFilter struct {
	Project  *uuid.UUID
	Kind     *ScanKind
	Statuses []ScanStatus
	Trigger  *ScanTrigger
}

func ValidateScanFilter(q ScanQuery) (ScanFilter, error) {
	var f ScanFilter
	if q.Project != "" {
		id, err := uuid.Parse(q.Project)
		if err != nil {
			return ScanFilter{}, InvalidScanFilter
		}
		f.Project = &id
	}
	switch k := ScanKind(q.Kind); k {
	case "":
	case CollectScan, IndexScan:
		f.Kind = &k
	default:
		return ScanFilter{}, InvalidScanFilter
	}
	switch t := ScanTrigger(q.Trigger); t {
	case "":
	case TriggerSchedule, TriggerManual:
		f.Trigger = &t
	default:
		return ScanFilter{}, InvalidScanFilter
	}
	if q.Status != "" {
		for _, raw := range strings.Split(q.Status, ",") {
			switch s := ScanStatus(strings.TrimSpace(raw)); s {
			case ScanOK, ScanUnchanged, ScanWarning, ScanFailed:
				f.Statuses = append(f.Statuses, s)
			default:
				return ScanFilter{}, InvalidScanFilter
			}
		}
	}
	return f, nil
}

func (s *Scan) Warn(code string) {
	for _, w := range s.Warnings {
		if w == code {
			return
		}
	}
	s.Warnings = append(s.Warnings, code)
}

func (s *Scan) Fail(code string, err error) {
	if s.Error != nil {
		return
	}
	detail := ""
	if err != nil {
		detail = RedactDetail(err.Error())
	}
	s.Error = &ScanError{Code: code, Detail: detail}
}

func (s *Scan) Settle() {
	collected := false
	for _, b := range s.Branches {
		if b.Result == BranchCollected && b.Files > 0 {
			collected = true
		}
	}
	if s.Index != nil && (s.Index.EmbeddedFiles > 0 || s.Index.Synced) {
		collected = true
	}
	switch {
	case s.Error != nil:
		s.Status = ScanFailed
	case collected:
		s.Status = ScanOK
	case len(s.Warnings) > 0:
		s.Status = ScanWarning
	default:
		s.Status = ScanUnchanged
	}
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`svc[rp]_[0-9A-Za-z]+`),
	regexp.MustCompile(`(?i)(bearer|token|basic)\s+[^\s,;"']+`),
	regexp.MustCompile(`(?i)((?:access_|private_|api_)?(?:token|key|secret|password)=)[^&\s"']+`),
	regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`),
}

func RedactDetail(s string) string {
	s = secretPatterns[0].ReplaceAllString(s, "[redacted]")
	s = secretPatterns[1].ReplaceAllString(s, "$1 [redacted]")
	s = secretPatterns[2].ReplaceAllString(s, "${1}[redacted]")
	s = secretPatterns[3].ReplaceAllString(s, "://[redacted]@")
	if r := []rune(s); len(r) > ScanDetailLimit {
		s = string(r[:ScanDetailLimit-1]) + "…"
	}
	return s
}

func IndexFailureCode(err error) string {
	switch {
	case errors.Is(err, ErrEmbeddingsDimensions):
		return "search.embeddings_dimensions"
	case errors.Is(err, ErrEmbeddingsUnavailable):
		return "search.embeddings_unavailable"
	case errors.Is(err, ErrEngineUnavailable), errors.Is(err, ErrSearchUnavailable):
		return "search.engine_unavailable"
	}
	return "internal"
}
