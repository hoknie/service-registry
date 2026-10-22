package services

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
)

type KnowledgeOverview struct {
	Synced   bool
	Source   knowledgetypes.SourceKind
	Settings knowledgetypes.Settings
	Branches []knowledgetypes.BranchState
}

type docProject struct {
	knowledgetypes.Project
	Source  *knowledgetypes.StoredSource
	Default string
}

func (s *DocumentsService) documentationOf(ctx context.Context, kp knowledgetypes.Project) (docProject, error) {
	src, err := s.sourceRepository.Get(ctx, kp.ID)
	if err != nil {
		return docProject{}, apperr.Wrap(err)
	}
	d := docProject{Project: kp, Source: src, Default: kp.DefaultBranch}
	switch {
	case src != nil && src.Kind == knowledgetypes.SourceLocalDir:
		d.Default = knowledgetypes.LocalBranch
	case src != nil && d.Default == "":
		d.Default = src.DefaultBranch
	}
	return d, nil
}

func (d docProject) kind() knowledgetypes.SourceKind {
	switch {
	case d.Source != nil:
		return d.Source.Kind
	case d.Synced:
		return knowledgetypes.SourceForge
	}
	return ""
}

func (s *DocumentsService) knowledgeProject(ctx context.Context, p accesstypes.Principal, perm catalogtypes.Permission, id uuid.UUID) (knowledgetypes.Project, error) {
	sc, err := s.catalog.Access.Authorize(ctx, p, perm, id)
	if err != nil {
		return knowledgetypes.Project{}, err
	}
	if sc.Node.Kind != catalogtypes.KindProject {
		return knowledgetypes.Project{}, apperr.New(apperr.NotFound)
	}
	kp, err := s.collectionRepository.Project(ctx, id)
	if err != nil {
		return knowledgetypes.Project{}, apperr.Wrap(err)
	}
	kp.Settings, err = s.settingsService.effectiveSettings(ctx, id)
	return kp, err
}

func (s *DocumentsService) GetKnowledge(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (KnowledgeOverview, error) {
	kp, err := s.knowledgeProject(ctx, p, catalogtypes.PermRead, id)
	if err != nil {
		return KnowledgeOverview{}, err
	}
	d, err := s.documentationOf(ctx, kp)
	if err != nil {
		return KnowledgeOverview{}, err
	}
	out := KnowledgeOverview{Synced: kp.Synced, Source: d.kind(), Settings: kp.Settings, Branches: []knowledgetypes.BranchState{}}
	var branches []knowledgetypes.Candidate
	if d.Source != nil {
		for name, sha := range d.Source.Heads {
			branches = append(branches, knowledgetypes.Candidate{Name: name, HeadSHA: sha})
		}
		if len(branches) == 0 && d.Default != "" {
			branches = append(branches, knowledgetypes.Candidate{Name: d.Default})
		}
	} else if branches, err = s.snapshotRepository.Branches(ctx, id); err != nil {
		return KnowledgeOverview{}, apperr.Wrap(err)
	}
	collecting := out.Source != ""
	for _, b := range branches {
		if !kp.Settings.CollectsBranch(b.Name, d.Default) {
			continue
		}
		bs := knowledgetypes.BranchState{Name: b.Name}
		if b.HeadSHA != "" {
			head := b.HeadSHA
			bs.HeadSHA = &head
		}
		if bs.Last, err = s.snapshotRepository.LastAttempt(ctx, id, b.Name); err != nil {
			return KnowledgeOverview{}, apperr.Wrap(err)
		}
		if bs.Snapshot, err = s.snapshotRepository.Latest(ctx, id, b.Name); err != nil {
			return KnowledgeOverview{}, apperr.Wrap(err)
		}
		bs.Pending = collecting && !b.Gone && (b.HeadSHA != "" || d.Source != nil) &&
			(kp.Force || bs.Last == nil || bs.Last.Commit != b.HeadSHA || bs.Last.Status == knowledgetypes.StatusFailed)
		out.Branches = append(out.Branches, bs)
	}
	slices.SortStableFunc(out.Branches, func(a, b knowledgetypes.BranchState) int {
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

type KnowledgeRef struct {
	Branch *string
	Commit *string
}

func (s *DocumentsService) resolveRef(ctx context.Context, kp knowledgetypes.Project, r KnowledgeRef) (string, knowledgetypes.Ref, error) {
	d, err := s.documentationOf(ctx, kp)
	if err != nil {
		return "", knowledgetypes.Ref{}, err
	}
	branch := d.Default
	if r.Branch != nil {
		name, err := catalogtypes.BranchName(*r.Branch)
		if err != nil {
			return "", knowledgetypes.Ref{}, apperr.Wrap(err)
		}
		branch = name
	}
	if branch == "" {
		return "", knowledgetypes.Ref{}, apperr.New(apperr.NotFound)
	}
	return branch, knowledgetypes.Ref{Commit: r.Commit}, nil
}

func (s *DocumentsService) KnowledgeFiles(ctx context.Context, p accesstypes.Principal, id uuid.UUID, r KnowledgeRef) (knowledgetypes.Snapshot, []knowledgetypes.FileInfo, error) {
	kp, err := s.knowledgeProject(ctx, p, catalogtypes.PermRead, id)
	if err != nil {
		return knowledgetypes.Snapshot{}, nil, err
	}
	branch, ref, err := s.resolveRef(ctx, kp, r)
	if err != nil {
		return knowledgetypes.Snapshot{}, nil, err
	}
	snap, files, err := s.snapshotRepository.Files(ctx, id, branch, ref)
	return snap, files, apperr.Wrap(err)
}

func (s *DocumentsService) KnowledgeFile(ctx context.Context, p accesstypes.Principal, id uuid.UUID, r KnowledgeRef, path string) (knowledgetypes.Snapshot, knowledgetypes.FileContent, error) {
	kp, err := s.knowledgeProject(ctx, p, catalogtypes.PermRead, id)
	if err != nil {
		return knowledgetypes.Snapshot{}, knowledgetypes.FileContent{}, err
	}
	branch, ref, err := s.resolveRef(ctx, kp, r)
	if err != nil {
		return knowledgetypes.Snapshot{}, knowledgetypes.FileContent{}, err
	}
	snap, f, err := s.snapshotRepository.File(ctx, id, branch, ref, path)
	return snap, f, apperr.Wrap(err)
}

func (s *DocumentsService) KnowledgeTree(ctx context.Context, p accesstypes.Principal, id uuid.UUID, r KnowledgeRef, rawDir string) (knowledgetypes.Snapshot, knowledgetypes.TreeLevel, error) {
	dir, ok := knowledgetypes.NormalizeDir(rawDir)
	if !ok {
		return knowledgetypes.Snapshot{}, knowledgetypes.TreeLevel{}, apperr.New(apperr.NotFound)
	}
	snap, files, err := s.KnowledgeFiles(ctx, p, id, r)
	if err != nil {
		return knowledgetypes.Snapshot{}, knowledgetypes.TreeLevel{}, err
	}
	level, found := knowledgetypes.Level(files, dir)
	if !found {
		return knowledgetypes.Snapshot{}, knowledgetypes.TreeLevel{}, apperr.New(apperr.NotFound)
	}
	return snap, level, nil
}

type DocumentsService struct {
	catalog              *catalogservices.CatalogServices
	collectionRepository *repositories.CollectionRepository
	snapshotRepository   *repositories.SnapshotRepository
	sourceRepository     *repositories.SourceRepository
	settingsService      *SettingsService
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

func (s *DocumentsService) KnowledgeOpenSpec(ctx context.Context, p accesstypes.Principal, id uuid.UUID, branch *string) (OpenSpec, error) {
	out := OpenSpec{Specs: []SpecItem{}, Changes: []ChangeItem{}, ADRs: []ADRItem{}}
	_, files, err := s.KnowledgeFiles(ctx, p, id, KnowledgeRef{Branch: branch})
	if apperr.From(err) != nil && apperr.From(err).Kind == apperr.NotFound {
		if _, err := s.knowledgeProject(ctx, p, catalogtypes.PermRead, id); err != nil {
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
		case knowledgetypes.KindSpec:
			reqs := f.Meta.Requirements
			if reqs == nil {
				reqs = []string{}
			}
			out.Specs = append(out.Specs, SpecItem{Capability: f.Meta.Capability, Path: f.Path, Purpose: f.Meta.Purpose, Requirements: reqs})
		case knowledgetypes.KindChange:
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
		case knowledgetypes.KindADR:
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
