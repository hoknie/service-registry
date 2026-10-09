package service

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
	"svc-registry/internal/knowledge"
)

type KnowledgeOverview struct {
	Synced   bool
	Source   knowledge.SourceKind
	Settings knowledge.Settings
	Branches []knowledge.BranchState
}

type docProject struct {
	knowledge.Project
	Source  *knowledge.StoredSource
	Default string
}

func documentationOf(ctx context.Context, state *State, kp knowledge.Project) (docProject, error) {
	src, err := state.Sources.Get(ctx, kp.ID)
	if err != nil {
		return docProject{}, apperr.Wrap(err)
	}
	d := docProject{Project: kp, Source: src, Default: kp.DefaultBranch}
	switch {
	case src != nil && src.Kind == knowledge.SourceLocalDir:
		d.Default = knowledge.LocalBranch
	case src != nil && d.Default == "":
		d.Default = src.DefaultBranch
	}
	return d, nil
}

func (d docProject) kind() knowledge.SourceKind {
	switch {
	case d.Source != nil:
		return d.Source.Kind
	case d.Synced:
		return knowledge.SourceForge
	}
	return ""
}

func knowledgeProject(ctx context.Context, state *State, p Principal, perm catalog.Permission, id uuid.UUID) (knowledge.Project, error) {
	s, err := Authorize(ctx, state, p, perm, id)
	if err != nil {
		return knowledge.Project{}, err
	}
	if s.Node.Kind != catalog.KindProject {
		return knowledge.Project{}, apperr.New(apperr.NotFound)
	}
	kp, err := state.Knowledge.Project(ctx, id)
	if err != nil {
		return knowledge.Project{}, apperr.Wrap(err)
	}
	kp.Settings, err = effectiveSettings(ctx, state, id)
	return kp, err
}

func effectiveSettings(ctx context.Context, state *State, id uuid.UUID) (knowledge.Settings, error) {
	chain, err := state.Patterns.Chain(ctx, id)
	if err != nil {
		return knowledge.Settings{}, apperr.Wrap(err)
	}
	s, _ := knowledge.Merge(chain)
	return s, nil
}

func GetKnowledge(ctx context.Context, state *State, p Principal, id uuid.UUID) (KnowledgeOverview, error) {
	kp, err := knowledgeProject(ctx, state, p, catalog.PermRead, id)
	if err != nil {
		return KnowledgeOverview{}, err
	}
	d, err := documentationOf(ctx, state, kp)
	if err != nil {
		return KnowledgeOverview{}, err
	}
	out := KnowledgeOverview{Synced: kp.Synced, Source: d.kind(), Settings: kp.Settings, Branches: []knowledge.BranchState{}}
	var branches []knowledge.Candidate
	if d.Source != nil {
		for name, sha := range d.Source.Heads {
			branches = append(branches, knowledge.Candidate{Name: name, HeadSHA: sha})
		}
		if len(branches) == 0 && d.Default != "" {
			branches = append(branches, knowledge.Candidate{Name: d.Default})
		}
	} else if branches, err = state.Snapshots.Branches(ctx, id); err != nil {
		return KnowledgeOverview{}, apperr.Wrap(err)
	}
	collecting := out.Source != ""
	for _, b := range branches {
		if !kp.Settings.CollectsBranch(b.Name, d.Default) {
			continue
		}
		bs := knowledge.BranchState{Name: b.Name}
		if b.HeadSHA != "" {
			head := b.HeadSHA
			bs.HeadSHA = &head
		}
		if bs.Last, err = state.Snapshots.LastAttempt(ctx, id, b.Name); err != nil {
			return KnowledgeOverview{}, apperr.Wrap(err)
		}
		if bs.Snapshot, err = state.Snapshots.Latest(ctx, id, b.Name); err != nil {
			return KnowledgeOverview{}, apperr.Wrap(err)
		}
		bs.Pending = collecting && !b.Gone && (b.HeadSHA != "" || d.Source != nil) &&
			(kp.Force || bs.Last == nil || bs.Last.Commit != b.HeadSHA || bs.Last.Status == knowledge.StatusFailed)
		out.Branches = append(out.Branches, bs)
	}
	slices.SortStableFunc(out.Branches, func(a, b knowledge.BranchState) int {
		switch {
		case a.Name == d.Default:
			return -1
		case b.Name == d.Default:
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}

type KnowledgeSettings struct {
	Settings                               knowledge.Settings
	Own                                    knowledge.NodeSettings
	IncludeFrom, ExcludeFrom, BranchesFrom *knowledge.ChainNode
}

func nodeSettings(ctx context.Context, state *State, id uuid.UUID) (KnowledgeSettings, error) {
	chain, err := state.Patterns.Chain(ctx, id)
	if err != nil {
		return KnowledgeSettings{}, apperr.Wrap(err)
	}
	s, from := knowledge.Merge(chain)
	origin := func(i int) *knowledge.ChainNode {
		if i <= 0 {
			return nil
		}
		return &chain[i]
	}
	return KnowledgeSettings{Settings: s, Own: chain[0].Own, IncludeFrom: origin(from.Include), ExcludeFrom: origin(from.Exclude),
		BranchesFrom: origin(from.Branches)}, nil
}

func GetKnowledgeSettings(ctx context.Context, state *State, p Principal, id uuid.UUID) (KnowledgeSettings, error) {
	if _, err := Authorize(ctx, state, p, catalog.PermRead, id); err != nil {
		return KnowledgeSettings{}, err
	}
	return nodeSettings(ctx, state, id)
}

func PutKnowledgeSettings(ctx context.Context, state *State, p Principal, id uuid.UUID, in knowledge.NodeSettings) (KnowledgeSettings, error) {
	if _, err := Authorize(ctx, state, p, catalog.PermWrite, id); err != nil {
		return KnowledgeSettings{}, err
	}
	s, err := knowledge.ValidateNodeSettings(in)
	if err != nil {
		return KnowledgeSettings{}, apperr.Wrap(err)
	}
	if err := state.Patterns.Put(ctx, id, s); err != nil {
		return KnowledgeSettings{}, apperr.Wrap(err)
	}
	return nodeSettings(ctx, state, id)
}

func CollectKnowledge(ctx context.Context, state *State, p Principal, id uuid.UUID) error {
	kp, err := knowledgeProject(ctx, state, p, catalog.PermWrite, id)
	if err != nil {
		return err
	}
	d, err := documentationOf(ctx, state, kp)
	if err != nil {
		return err
	}
	if d.kind() == "" {
		return apperr.Wrap(knowledge.ConflictNotSynced)
	}
	return apperr.Wrap(state.Knowledge.RequestCollect(ctx, id))
}

type KnowledgeRef struct {
	Branch *string
	Commit *string
}

func resolveRef(ctx context.Context, state *State, kp knowledge.Project, r KnowledgeRef) (string, knowledge.Ref, error) {
	d, err := documentationOf(ctx, state, kp)
	if err != nil {
		return "", knowledge.Ref{}, err
	}
	branch := d.Default
	if r.Branch != nil {
		name, err := catalog.BranchName(*r.Branch)
		if err != nil {
			return "", knowledge.Ref{}, apperr.Wrap(err)
		}
		branch = name
	}
	if branch == "" {
		return "", knowledge.Ref{}, apperr.New(apperr.NotFound)
	}
	return branch, knowledge.Ref{Commit: r.Commit}, nil
}

func KnowledgeFiles(ctx context.Context, state *State, p Principal, id uuid.UUID, r KnowledgeRef) (knowledge.Snapshot, []knowledge.FileInfo, error) {
	kp, err := knowledgeProject(ctx, state, p, catalog.PermRead, id)
	if err != nil {
		return knowledge.Snapshot{}, nil, err
	}
	branch, ref, err := resolveRef(ctx, state, kp, r)
	if err != nil {
		return knowledge.Snapshot{}, nil, err
	}
	snap, files, err := state.Snapshots.Files(ctx, id, branch, ref)
	return snap, files, apperr.Wrap(err)
}

func KnowledgeFile(ctx context.Context, state *State, p Principal, id uuid.UUID, r KnowledgeRef, path string) (knowledge.Snapshot, knowledge.FileContent, error) {
	kp, err := knowledgeProject(ctx, state, p, catalog.PermRead, id)
	if err != nil {
		return knowledge.Snapshot{}, knowledge.FileContent{}, err
	}
	branch, ref, err := resolveRef(ctx, state, kp, r)
	if err != nil {
		return knowledge.Snapshot{}, knowledge.FileContent{}, err
	}
	snap, f, err := state.Snapshots.File(ctx, id, branch, ref, path)
	return snap, f, apperr.Wrap(err)
}

type SpecItem struct {
	Capability, Path, Purpose string
	Requirements              []string
}

type ChangeItem struct {
	ID, Path  string
	Archived  bool
	Artifacts []string
}

type ADRItem struct {
	Number                          int
	Title, Status, Supersedes, Path string
}

type OpenSpec struct {
	Specs   []SpecItem
	Changes []ChangeItem
	ADRs    []ADRItem
}

func KnowledgeOpenSpec(ctx context.Context, state *State, p Principal, id uuid.UUID, branch *string) (OpenSpec, error) {
	out := OpenSpec{Specs: []SpecItem{}, Changes: []ChangeItem{}, ADRs: []ADRItem{}}
	_, files, err := KnowledgeFiles(ctx, state, p, id, KnowledgeRef{Branch: branch})
	if apperr.From(err) != nil && apperr.From(err).Kind == apperr.NotFound {
		if _, err := knowledgeProject(ctx, state, p, catalog.PermRead, id); err != nil {
			return OpenSpec{}, err
		}
		return out, nil
	}
	if err != nil {
		return OpenSpec{}, err
	}
	changes := map[string]*ChangeItem{}
	var order []string
	for _, f := range files {
		switch f.Kind {
		case knowledge.KindSpec:
			reqs := f.Meta.Requirements
			if reqs == nil {
				reqs = []string{}
			}
			out.Specs = append(out.Specs, SpecItem{Capability: f.Meta.Capability, Path: f.Path, Purpose: f.Meta.Purpose, Requirements: reqs})
		case knowledge.KindChange:
			key := f.Meta.Change
			if f.Meta.Archived {
				key = "archive/" + key
			}
			c := changes[key]
			if c == nil {
				prefix := "openspec/changes/" + key
				c = &ChangeItem{ID: f.Meta.Change, Path: prefix, Archived: f.Meta.Archived, Artifacts: []string{}}
				changes[key] = c
				order = append(order, key)
			}
			c.Artifacts = append(c.Artifacts, strings.TrimPrefix(f.Path, c.Path+"/"))
		case knowledge.KindADR:
			out.ADRs = append(out.ADRs, ADRItem{Number: f.Meta.Number, Title: f.Meta.Title, Status: f.Meta.Status,
				Supersedes: f.Meta.Supersedes, Path: f.Path})
		}
	}
	slices.SortFunc(out.Specs, func(a, b SpecItem) int { return strings.Compare(a.Capability, b.Capability) })
	for _, k := range order {
		out.Changes = append(out.Changes, *changes[k])
	}
	slices.SortStableFunc(out.Changes, func(a, b ChangeItem) int {
		switch {
		case a.Archived != b.Archived && !a.Archived:
			return -1
		case a.Archived != b.Archived:
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	slices.SortFunc(out.ADRs, func(a, b ADRItem) int { return a.Number - b.Number })
	return out, nil
}
