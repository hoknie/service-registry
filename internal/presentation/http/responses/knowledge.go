package responses

import (
	"github.com/google/uuid"

	"svc-registry/internal/feature/knowledge"
	knowledgeservice "svc-registry/internal/feature/knowledge/service"
)

type KnowledgeSettings struct {
	Include  []string `json:"include"`
	Exclude  []string `json:"exclude"`
	Branches []string `json:"branches"`
}

type NodeSettings struct {
	KnowledgeSettings
	Own  map[string]any             `json:"own"`
	From map[string]*SettingsOrigin `json:"from"`
}

type SettingsOrigin struct {
	ID   uuid.UUID `json:"id"`
	Kind string    `json:"kind"`
	Name string    `json:"name"`
}

func NodeSettingsOf(s knowledgeservice.KnowledgeSettings) NodeSettings {
	own := map[string]any{}
	if s.Own.IncludeSet {
		own["include"] = s.Own.Include
	}
	if s.Own.Exclude != nil {
		own["exclude"] = *s.Own.Exclude
	}
	if s.Own.Branches != nil {
		own["branches"] = *s.Own.Branches
	}
	origin := func(n *knowledge.ChainNode) *SettingsOrigin {
		if n == nil {
			return nil
		}
		return &SettingsOrigin{ID: n.ID, Kind: n.Kind, Name: n.Name}
	}
	return NodeSettings{KnowledgeSettings: KnowledgeSettingsOf(s.Settings), Own: own,
		From: map[string]*SettingsOrigin{"include": origin(s.IncludeFrom), "exclude": origin(s.ExcludeFrom), "branches": origin(s.BranchesFrom)}}
}

func KnowledgeSettingsOf(s knowledge.Settings) KnowledgeSettings {
	out := KnowledgeSettings{Include: s.Include, Exclude: s.Exclude, Branches: s.Branches}
	if out.Exclude == nil {
		out.Exclude = []string{}
	}
	if out.Branches == nil {
		out.Branches = []string{}
	}
	return out
}

type Snapshot struct {
	ID          uuid.UUID `json:"id"`
	Branch      string    `json:"branch"`
	Commit      string    `json:"commit"`
	Status      string    `json:"status"`
	ErrorCode   *string   `json:"error_code"`
	Files       int       `json:"files"`
	Bytes       int64     `json:"bytes"`
	Skipped     int       `json:"skipped"`
	Truncated   bool      `json:"truncated"`
	CollectedAt string    `json:"collected_at"`
}

func SnapshotOf(s *knowledge.Snapshot) *Snapshot {
	if s == nil {
		return nil
	}
	return &Snapshot{ID: s.ID, Branch: s.Branch, Commit: s.Commit, Status: string(s.Status), ErrorCode: s.ErrorCode,
		Files: s.Files, Bytes: s.Bytes, Skipped: s.Skipped, Truncated: s.Truncated, CollectedAt: s.CollectedAt}
}

type KnowledgeBranch struct {
	Name     string    `json:"name"`
	HeadSHA  *string   `json:"head_sha"`
	Pending  bool      `json:"pending"`
	Last     *Snapshot `json:"last"`
	Snapshot *Snapshot `json:"snapshot"`
}

type Knowledge struct {
	Synced   bool              `json:"synced"`
	Source   *string           `json:"source"`
	Settings KnowledgeSettings `json:"settings"`
	Branches []KnowledgeBranch `json:"branches"`
}

func KnowledgeOf(o knowledgeservice.KnowledgeOverview) Knowledge {
	out := Knowledge{Synced: o.Synced, Settings: KnowledgeSettingsOf(o.Settings), Branches: make([]KnowledgeBranch, 0, len(o.Branches))}
	if o.Source != "" {
		src := string(o.Source)
		out.Source = &src
	}
	for _, b := range o.Branches {
		out.Branches = append(out.Branches, KnowledgeBranch{Name: b.Name, HeadSHA: b.HeadSHA, Pending: b.Pending,
			Last: SnapshotOf(b.Last), Snapshot: SnapshotOf(b.Snapshot)})
	}
	return out
}

type KnowledgeFile struct {
	Path       string  `json:"path"`
	Kind       string  `json:"kind"`
	Bytes      int64   `json:"bytes"`
	SkipReason *string `json:"skip_reason"`
}

func fileOf(f knowledge.FileInfo) KnowledgeFile {
	out := KnowledgeFile{Path: f.Path, Kind: string(f.Kind), Bytes: f.Bytes}
	if f.Skip != nil {
		s := string(*f.Skip)
		out.SkipReason = &s
	}
	return out
}

type KnowledgeFiles struct {
	Snapshot *Snapshot       `json:"snapshot"`
	Items    []KnowledgeFile `json:"items"`
}

func KnowledgeFilesOf(s knowledge.Snapshot, files []knowledge.FileInfo) KnowledgeFiles {
	out := KnowledgeFiles{Snapshot: SnapshotOf(&s), Items: make([]KnowledgeFile, 0, len(files))}
	for _, f := range files {
		out.Items = append(out.Items, fileOf(f))
	}
	return out
}

type KnowledgeFileContent struct {
	Snapshot *Snapshot `json:"snapshot"`
	KnowledgeFile
	Content *string `json:"content"`
}

func KnowledgeFileContentOf(s knowledge.Snapshot, f knowledge.FileContent) KnowledgeFileContent {
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

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func SpecsOf(o knowledgeservice.OpenSpec) Items[Spec] {
	out := make([]Spec, 0, len(o.Specs))
	for _, s := range o.Specs {
		out = append(out, Spec{Capability: s.Capability, Path: s.Path, Purpose: s.Purpose, Requirements: s.Requirements})
	}
	return Items[Spec]{Items: out}
}

func ChangesOf(o knowledgeservice.OpenSpec) Items[Change] {
	out := make([]Change, 0, len(o.Changes))
	for _, c := range o.Changes {
		out = append(out, Change{ID: c.ID, Archived: c.Archived, Path: c.Path, Artifacts: c.Artifacts})
	}
	return Items[Change]{Items: out}
}

func ADRsOf(o knowledgeservice.OpenSpec) Items[ADR] {
	out := make([]ADR, 0, len(o.ADRs))
	for _, a := range o.ADRs {
		out = append(out, ADR{Number: a.Number, Title: a.Title, Status: nonEmpty(a.Status), Supersedes: nonEmpty(a.Supersedes), Path: a.Path})
	}
	return Items[ADR]{Items: out}
}

type SearchProject struct {
	ID   uuid.UUID `json:"id"`
	Path string    `json:"path"`
	Name string    `json:"name"`
}

type SearchHit struct {
	Project SearchProject       `json:"project"`
	Branch  string              `json:"branch"`
	Commit  string              `json:"commit"`
	Path    string              `json:"path"`
	Kind    string              `json:"kind"`
	Snippet []knowledge.Segment `json:"snippet"`
}

type Search struct {
	Items      []SearchHit `json:"items"`
	NextCursor *string     `json:"next_cursor"`
}

func SearchOf(r knowledgeservice.SearchResult) Search {
	out := Search{Items: make([]SearchHit, 0, len(r.Hits))}
	for _, h := range r.Hits {
		snippet := h.Snippet
		if snippet == nil {
			snippet = []knowledge.Segment{}
		}
		out.Items = append(out.Items, SearchHit{Project: SearchProject{ID: h.ProjectID, Path: h.ProjectPath, Name: h.ProjectName},
			Branch: h.Branch, Commit: h.Commit, Path: h.Path, Kind: string(h.Kind), Snippet: snippet})
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

func SearchModesOf(m knowledgeservice.SearchModes) SearchModes {
	out := SearchModes{Engine: m.Engine, Default: string(m.Default), Modes: make([]string, len(m.Modes))}
	for i, x := range m.Modes {
		out.Modes[i] = string(x)
	}
	if m.Pending != nil && m.Indexed != nil {
		out.Index = &IndexStatus{Pending: *m.Pending, Indexed: *m.Indexed}
	}
	return out
}

type SourceCredentials struct {
	Mode        string  `json:"mode"`
	Fingerprint *string `json:"fingerprint"`
}

type KnowledgeSource struct {
	Kind           string            `json:"kind"`
	Forge          *string           `json:"forge"`
	URL            *string           `json:"url"`
	APIURL         *string           `json:"api_url"`
	Path           *string           `json:"path"`
	Credentials    SourceCredentials `json:"credentials"`
	WorkingTree    bool              `json:"working_tree"`
	IncludeIgnored bool              `json:"include_ignored"`
	UpdatedAt      string            `json:"updated_at"`
}

func KnowledgeSourceOf(s *knowledge.Source) *KnowledgeSource {
	if s == nil {
		return nil
	}
	return &KnowledgeSource{Kind: string(s.Kind), Forge: nonEmpty(s.Forge), URL: nonEmpty(s.URL), APIURL: nonEmpty(s.APIURL),
		Path: nonEmpty(s.Path), Credentials: SourceCredentials{Mode: s.Credentials.Mode, Fingerprint: s.Credentials.Fingerprint},
		WorkingTree: s.WorkingTree, IncludeIgnored: s.IncludeIgnored, UpdatedAt: s.UpdatedAt}
}

type SourceCheck struct {
	OK        bool    `json:"ok"`
	Branches  *int    `json:"branches,omitempty"`
	ErrorCode *string `json:"error_code,omitempty"`
}

func SourceCheckOf(c knowledgeservice.SourceCheck) SourceCheck {
	if c.OK {
		return SourceCheck{OK: true, Branches: &c.Branches}
	}
	return SourceCheck{ErrorCode: &c.ErrorCode}
}
