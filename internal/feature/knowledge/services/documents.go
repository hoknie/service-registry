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
	"svc-registry/internal/platform/config"
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
	kp.Editor = sc.Allows(catalogtypes.PermWrite)
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
			bs.HeadSHA = new(b.HeadSHA)
		}
		if bs.Last, err = s.snapshotRepository.LastAttempt(ctx, id, b.Name); err != nil {
			return KnowledgeOverview{}, apperr.Wrap(err)
		}
		if bs.Snapshot, err = s.snapshotRepository.Latest(ctx, id, b.Name); err != nil {
			return KnowledgeOverview{}, apperr.Wrap(err)
		}
		s.explain(kp, bs.Last)
		s.explain(kp, bs.Snapshot)
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
	s.explain(kp, &snap)
	return snap, files, apperr.Wrap(err)
}

func (s *DocumentsService) explain(kp knowledgetypes.Project, snap *knowledgetypes.Snapshot) {
	if snap != nil && kp.Editor {
		k := s.cfg.Knowledge
		snap.Limits = knowledgetypes.Limits{MaxFileBytes: k.MaxFileBytes, MaxFiles: int(k.MaxFiles), MaxSnapshotBytes: k.MaxSnapshotBytes}.Reached(*snap)
	}
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
	s.explain(kp, &snap)
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

func (s *DocumentsService) SnapshotContents(ctx context.Context, p accesstypes.Principal, id uuid.UUID, r KnowledgeRef) (knowledgetypes.SnapshotFiles, error) {
	snap, files, err := s.KnowledgeFiles(ctx, p, id, r)
	if err != nil {
		return knowledgetypes.SnapshotFiles{}, err
	}
	return knowledgetypes.SnapshotFiles{Snapshot: snap, Files: files, Contents: func(ctx context.Context, paths []string) (map[string]string, error) {
		out, err := s.snapshotRepository.Contents(ctx, snap.ID, paths)
		return out, apperr.Wrap(err)
	}}, nil
}

type DocumentsService struct {
	catalog              *catalogservices.CatalogServices
	collectionRepository *repositories.CollectionRepository
	snapshotRepository   *repositories.SnapshotRepository
	sourceRepository     *repositories.SourceRepository
	settingsService      *SettingsService
	cfg                  *config.Config
}
