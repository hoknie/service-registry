package responses

import (
	"github.com/google/uuid"

	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/pkg/ptr"
)

type Snapshot struct {
	ID          uuid.UUID `json:"id"`
	Branch      string    `json:"branch"`
	RefKind     string    `json:"ref_kind"`
	Commit      string    `json:"commit"`
	Status      string    `json:"status"`
	ErrorCode   *string   `json:"error_code"`
	Files       int       `json:"files"`
	Bytes       int64     `json:"bytes"`
	Skipped     int       `json:"skipped"`
	Limited     int       `json:"skipped_limit"`
	TooLarge    int       `json:"skipped_too_large"`
	Truncated   bool      `json:"truncated"`
	Limits      []Limit   `json:"limits"`
	CollectedAt string    `json:"collected_at"`
}

func SnapshotOf(s *knowledgetypes.Snapshot) *Snapshot {
	if s == nil {
		return nil
	}
	kind := string(s.RefKind)
	if kind == "" {
		kind = string(knowledgetypes.RefBranch)
	}
	return &Snapshot{ID: s.ID, Branch: s.Branch, RefKind: kind, Commit: s.Commit, Status: string(s.Status), ErrorCode: s.ErrorCode,
		Files: s.Files, Bytes: s.Bytes, Skipped: s.Skipped, Limited: s.Limited, TooLarge: s.TooLarge, Truncated: s.Truncated,
		Limits: limitsOf(s.Limits), CollectedAt: s.CollectedAt}
}

type KnowledgeBranch struct {
	Name     string    `json:"name"`
	HeadSHA  *string   `json:"head_sha"`
	Pending  bool      `json:"pending"`
	Last     *Snapshot `json:"last"`
	Snapshot *Snapshot `json:"snapshot"`
}

type KnowledgeTag struct {
	Name     string    `json:"name"`
	Commit   string    `json:"commit"`
	Release  bool      `json:"release"`
	Pending  bool      `json:"pending"`
	Last     *Snapshot `json:"last"`
	Snapshot *Snapshot `json:"snapshot"`
}

type Knowledge struct {
	Synced   bool              `json:"synced"`
	Source   *string           `json:"source"`
	Settings KnowledgeSettings `json:"settings"`
	Branches []KnowledgeBranch `json:"branches"`
	Tags     []KnowledgeTag    `json:"tags"`
	Release  *string           `json:"release"`
}

func KnowledgeOf(o knowledgeservices.KnowledgeOverview) Knowledge {
	out := Knowledge{Synced: o.Synced, Settings: KnowledgeSettingsOf(o.Settings), Branches: make([]KnowledgeBranch, 0, len(o.Branches)),
		Tags: make([]KnowledgeTag, 0, len(o.Tags)), Release: o.Release}
	if o.Source != "" {
		out.Source = new(string(o.Source))
	}
	for _, b := range o.Branches {
		out.Branches = append(out.Branches, KnowledgeBranch{Name: b.Name, HeadSHA: b.HeadSHA, Pending: b.Pending,
			Last: SnapshotOf(b.Last), Snapshot: SnapshotOf(b.Snapshot)})
	}
	for _, t := range o.Tags {
		out.Tags = append(out.Tags, KnowledgeTag{Name: t.Name, Commit: ptr.Value(t.HeadSHA), Release: t.Release, Pending: t.Pending,
			Last: SnapshotOf(t.Last), Snapshot: SnapshotOf(t.Snapshot)})
	}
	return out
}

type KnowledgeFile struct {
	Path       string  `json:"path"`
	Kind       string  `json:"kind"`
	Bytes      int64   `json:"bytes"`
	SkipReason *string `json:"skip_reason"`
	GitStatus  *string `json:"git_status"`
}

func fileOf(f knowledgetypes.FileInfo) KnowledgeFile {
	out := KnowledgeFile{Path: f.Path, Kind: string(f.Kind), Bytes: f.Bytes}
	if f.Meta.Git != "" {
		out.GitStatus = new(string(f.Meta.Git))
	}
	if f.Skip != nil {
		out.SkipReason = new(string(*f.Skip))
	}
	return out
}

type KnowledgeFiles struct {
	Snapshot *Snapshot       `json:"snapshot"`
	Items    []KnowledgeFile `json:"items"`
}

func KnowledgeFilesOf(s knowledgetypes.Snapshot, files []knowledgetypes.FileInfo) KnowledgeFiles {
	out := KnowledgeFiles{Snapshot: SnapshotOf(&s), Items: make([]KnowledgeFile, 0, len(files))}
	for _, f := range files {
		out.Items = append(out.Items, fileOf(f))
	}
	return out
}

type KnowledgeTreeDir struct {
	Name      string  `json:"name"`
	Path      string  `json:"path"`
	Files     int     `json:"files"`
	GitStatus *string `json:"git_status"`
}

type KnowledgeTree struct {
	Snapshot *Snapshot           `json:"snapshot"`
	Dirs     []KnowledgeTreeDir  `json:"dirs"`
	Files    []KnowledgeTreeFile `json:"files"`
}

type KnowledgeTreeFile struct {
	KnowledgeFile
	Title *string `json:"title"`
}

func treeFileOf(f knowledgetypes.FileInfo) KnowledgeTreeFile {
	return KnowledgeTreeFile{KnowledgeFile: fileOf(f), Title: ptr.NonEmpty(f.Title)}
}

func KnowledgeTreeOf(s knowledgetypes.Snapshot, l knowledgetypes.TreeLevel) KnowledgeTree {
	out := KnowledgeTree{Snapshot: SnapshotOf(&s), Dirs: make([]KnowledgeTreeDir, 0, len(l.Dirs)), Files: make([]KnowledgeTreeFile, 0, len(l.Files))}
	for _, d := range l.Dirs {
		dir := KnowledgeTreeDir{Name: d.Name, Path: d.Path, Files: d.Files}
		if d.Git != "" {
			dir.GitStatus = new(string(d.Git))
		}
		out.Dirs = append(out.Dirs, dir)
	}
	for _, f := range l.Files {
		out.Files = append(out.Files, treeFileOf(f))
	}
	return out
}

type KnowledgeFileContent struct {
	Snapshot *Snapshot `json:"snapshot"`
	KnowledgeFile
	Content *string `json:"content"`
}

func KnowledgeFileContentOf(s knowledgetypes.Snapshot, f knowledgetypes.FileContent) KnowledgeFileContent {
	return KnowledgeFileContent{Snapshot: SnapshotOf(&s), KnowledgeFile: fileOf(f.FileInfo), Content: f.Content}
}

type Spec struct {
	Capability   string   `json:"capability"`
	Path         string   `json:"path"`
	Purpose      string   `json:"purpose"`
	Requirements []string `json:"requirements"`
}

type Change struct {
	ID        string   `json:"id"`
	Archived  bool     `json:"archived"`
	Path      string   `json:"path"`
	Artifacts []string `json:"artifacts"`
}

type ADR struct {
	Number     int     `json:"number"`
	Title      string  `json:"title"`
	Status     *string `json:"status"`
	Supersedes *string `json:"supersedes"`
	Path       string  `json:"path"`
}

func SpecsOf(o knowledgeservices.OpenSpec) Items[Spec] {
	out := make([]Spec, 0, len(o.Specs))
	for _, s := range o.Specs {
		out = append(out, Spec{Capability: s.Capability, Path: s.Path, Purpose: s.Purpose, Requirements: s.Requirements})
	}
	return Items[Spec]{Items: out}
}

func ChangesOf(o knowledgeservices.OpenSpec) Items[Change] {
	out := make([]Change, 0, len(o.Changes))
	for _, c := range o.Changes {
		out = append(out, Change{ID: c.ID, Archived: c.Archived, Path: c.Path, Artifacts: c.Artifacts})
	}
	return Items[Change]{Items: out}
}

func ADRsOf(o knowledgeservices.OpenSpec) Items[ADR] {
	out := make([]ADR, 0, len(o.ADRs))
	for _, a := range o.ADRs {
		out = append(out, ADR{Number: a.Number, Title: a.Title, Status: ptr.NonEmpty(a.Status), Supersedes: ptr.NonEmpty(a.Supersedes), Path: a.Path})
	}
	return Items[ADR]{Items: out}
}

type SearchProject struct {
	ID   uuid.UUID `json:"id"`
	Path string    `json:"path"`
	Name string    `json:"name"`
}

type SearchHit struct {
	Project SearchProject            `json:"project"`
	Branch  string                   `json:"branch"`
	RefKind string                   `json:"ref_kind"`
	Commit  string                   `json:"commit"`
	Path    string                   `json:"path"`
	Kind    string                   `json:"kind"`
	Snippet []knowledgetypes.Segment `json:"snippet"`
}

type Search struct {
	Items      []SearchHit `json:"items"`
	NextCursor *string     `json:"next_cursor"`
}

func SearchOf(r knowledgeservices.SearchResult) Search {
	out := Search{Items: make([]SearchHit, 0, len(r.Hits))}
	for _, h := range r.Hits {
		snippet := h.Snippet
		if snippet == nil {
			snippet = []knowledgetypes.Segment{}
		}
		out.Items = append(out.Items, SearchHit{Project: SearchProject{ID: h.ProjectID, Path: h.ProjectPath, Name: h.ProjectName},
			Branch: h.Branch, RefKind: string(h.RefKind), Commit: h.Commit, Path: h.Path, Kind: string(h.Kind), Snippet: snippet})
	}
	if r.Next != "" {
		out.NextCursor = &r.Next
	}
	return out
}

type SearchModes struct {
	Engine  string       `json:"engine"`
	Modes   []string     `json:"modes"`
	Default string       `json:"default"`
	Index   *IndexStatus `json:"index"`
}

type IndexStatus struct {
	Pending int `json:"pending"`
	Indexed int `json:"indexed"`
}

func SearchModesOf(m knowledgeservices.SearchModes) SearchModes {
	out := SearchModes{Engine: m.Engine, Default: string(m.Default), Modes: make([]string, len(m.Modes))}
	for i, x := range m.Modes {
		out.Modes[i] = string(x)
	}
	if m.Pending != nil && m.Indexed != nil {
		out.Index = &IndexStatus{Pending: *m.Pending, Indexed: *m.Indexed}
	}
	return out
}
