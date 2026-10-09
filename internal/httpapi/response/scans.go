package response

import (
	"github.com/google/uuid"

	"svc-registry/internal/knowledge"
)

type ScanProject struct {
	ID   uuid.UUID `json:"id"`
	Path string    `json:"path"`
	Name string    `json:"name"`
}

type ScanBranch struct {
	Name        string         `json:"name"`
	Commit      string         `json:"commit"`
	Result      string         `json:"result"`
	Files       int            `json:"files"`
	Skipped     map[string]int `json:"skipped"`
	Truncated   bool           `json:"truncated"`
	WorkingTree bool           `json:"working_tree"`
	Error       *string        `json:"error"`
}

type ScanIndex struct {
	EmbeddedFiles  int    `json:"embedded_files"`
	EmbeddedChunks int    `json:"embedded_chunks"`
	Documents      int    `json:"documents"`
	Engine         string `json:"engine"`
	Model          string `json:"model"`
}

type ScanError struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

type Scan struct {
	ID         uuid.UUID    `json:"id"`
	Project    ScanProject  `json:"project"`
	Kind       string       `json:"kind"`
	Trigger    string       `json:"trigger"`
	Source     *string      `json:"source"`
	Status     string       `json:"status"`
	StartedAt  string       `json:"started_at"`
	FinishedAt string       `json:"finished_at"`
	DurationMS int64        `json:"duration_ms"`
	Repeats    int          `json:"repeats"`
	Branches   []ScanBranch `json:"branches"`
	Index      *ScanIndex   `json:"index"`
	Error      *ScanError   `json:"error"`
	Warnings   []string     `json:"warnings"`
}

func ScanOf(s knowledge.ScanItem) Scan {
	branches := make([]ScanBranch, 0, len(s.Branches))
	for _, b := range s.Branches {
		out := ScanBranch{Name: b.Name, Commit: b.Commit, Result: string(b.Result), Files: b.Files, Skipped: b.Skipped, Truncated: b.Truncated,
			WorkingTree: b.WorkingTree}
		if out.Skipped == nil {
			out.Skipped = map[string]int{}
		}
		if b.Error != "" {
			code := b.Error
			out.Error = &code
		}
		branches = append(branches, out)
	}
	var index *ScanIndex
	if s.Index != nil {
		index = &ScanIndex{EmbeddedFiles: s.Index.EmbeddedFiles, EmbeddedChunks: s.Index.EmbeddedChunks, Documents: s.Index.Documents,
			Engine: s.Index.Engine, Model: s.Index.Model}
	}
	var scanErr *ScanError
	if s.Error != nil {
		scanErr = &ScanError{Code: s.Error.Code, Detail: s.Error.Detail}
	}
	warnings := s.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return Scan{ID: s.ID, Project: ScanProject{ID: s.ProjectID, Path: s.ProjectPath, Name: s.ProjectName}, Kind: string(s.Kind),
		Trigger: string(s.Trigger), Source: s.Source, Status: string(s.Status), StartedAt: s.StartedAt, FinishedAt: s.FinishedAt,
		DurationMS: s.DurationMS, Repeats: s.Repeats, Branches: branches, Index: index, Error: scanErr, Warnings: warnings}
}
