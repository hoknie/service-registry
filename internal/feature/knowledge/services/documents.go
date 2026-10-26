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
	Tags     []knowledgetypes.BranchState
	Release  *string
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
		if err := s.fillState(ctx, kp, knowledgetypes.RefBranch, &bs); err != nil {
			return KnowledgeOverview{}, err
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
	if out.Tags, out.Release, err = s.tagStates(ctx, kp, collecting); err != nil {
		return KnowledgeOverview{}, err
	}
	return out, nil
}

func (s *DocumentsService) fillState(ctx context.Context, kp knowledgetypes.Project, kind knowledgetypes.RefKind, bs *knowledgetypes.BranchState) error {
	var err error
	if bs.Last, err = s.snapshotRepository.LastAttempt(ctx, kp.ID, kind, bs.Name); err != nil {
		return apperr.Wrap(err)
	}
	if bs.Snapshot, err = s.snapshotRepository.Latest(ctx, kp.ID, kind, bs.Name); err != nil {
		return apperr.Wrap(err)
	}
	s.explain(kp, bs.Last)
	s.explain(kp, bs.Snapshot)
	return nil
}

func (s *DocumentsService) tagStates(ctx context.Context, kp knowledgetypes.Project, collecting bool) ([]knowledgetypes.BranchState, *string, error) {
	tags, err := s.tagRepository.List(ctx, kp.ID)
	if err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	knowledgetypes.SortTags(tags, func(t knowledgetypes.StoredTag) knowledgetypes.TagInfo {
		return knowledgetypes.TagInfo{Name: t.Name, FirstSeen: t.FirstSeen}
	})
	out := make([]knowledgetypes.BranchState, 0, len(tags))
	var release *string
	for _, t := range tags {
		bs := knowledgetypes.BranchState{Name: t.Name, HeadSHA: new(t.Commit), Release: t.Release}
		if err := s.fillState(ctx, kp, knowledgetypes.RefTag, &bs); err != nil {
			return nil, nil, err
		}
		bs.Pending = collecting && (kp.Force || bs.Last == nil || bs.Last.Commit != t.Commit || bs.Last.Status == knowledgetypes.StatusFailed)
		if t.Release {
			release = new(t.Name)
		}
		out = append(out, bs)
	}
	return out, release, nil
}

type KnowledgeRef struct {
	Branch  *string
	Tag     *string
	Release *string
	Commit  *string
}

func (s *DocumentsService) resolveRef(ctx context.Context, kp knowledgetypes.Project, r KnowledgeRef) (knowledgetypes.RefSelector, knowledgetypes.Ref, error) {
	sel, err := knowledgetypes.SelectRef(r.Branch, r.Tag, r.Release)
	if err != nil {
		return sel, knowledgetypes.Ref{}, apperr.Wrap(err)
	}
	ref := knowledgetypes.Ref{Commit: r.Commit}
	switch {
	case sel.Release:
		name, err := s.tagRepository.Release(ctx, kp.ID)
		if err != nil {
			return sel, ref, apperr.Wrap(err)
		}
		if name == nil {
			return sel, ref, apperr.ErrNotFound
		}
		sel.Name = *name
	case sel.Kind == knowledgetypes.RefTag:
		found, err := s.tagRepository.Has(ctx, kp.ID, sel.Name)
		if err != nil {
			return sel, ref, apperr.Wrap(err)
		}
		if !found {
			return sel, ref, apperr.ErrNotFound
		}
	case sel.IsZero():
		d, err := s.documentationOf(ctx, kp)
		if err != nil {
			return sel, ref, err
		}
		sel = knowledgetypes.RefSelector{Kind: knowledgetypes.RefBranch, Name: d.Default}
	}
	if sel.Name == "" {
		return sel, ref, apperr.New(apperr.NotFound)
	}
	return sel, ref, nil
}

func (s *DocumentsService) KnowledgeFiles(ctx context.Context, p accesstypes.Principal, id uuid.UUID, r KnowledgeRef) (knowledgetypes.Snapshot, []knowledgetypes.FileInfo, error) {
	kp, err := s.knowledgeProject(ctx, p, catalogtypes.PermRead, id)
	if err != nil {
		return knowledgetypes.Snapshot{}, nil, err
	}
	sel, ref, err := s.resolveRef(ctx, kp, r)
	if err != nil {
		return knowledgetypes.Snapshot{}, nil, err
	}
	snap, files, err := s.snapshotFiles(ctx, id, sel, ref)
	s.explain(kp, &snap)
	return snap, files, apperr.Wrap(err)
}

func (s *DocumentsService) resolveSnapshot(ctx context.Context, id uuid.UUID, sel knowledgetypes.RefSelector, ref knowledgetypes.Ref) (knowledgetypes.Snapshot, error) {
	snap, err := s.snapshotRepository.Resolve(ctx, id, sel.Kind, sel.Name, ref.Commit)
	if err != nil {
		return knowledgetypes.Snapshot{}, err
	}
	if snap == nil {
		return knowledgetypes.Snapshot{}, apperr.ErrNotFound
	}
	return *snap, nil
}

func (s *DocumentsService) snapshotFiles(ctx context.Context, id uuid.UUID, sel knowledgetypes.RefSelector, ref knowledgetypes.Ref) (knowledgetypes.Snapshot, []knowledgetypes.FileInfo, error) {
	snap, err := s.resolveSnapshot(ctx, id, sel, ref)
	if err != nil {
		return knowledgetypes.Snapshot{}, nil, err
	}
	files, err := s.snapshotRepository.Files(ctx, snap.ID)
	if err != nil {
		return knowledgetypes.Snapshot{}, nil, err
	}
	return snap, files, nil
}

func (s *DocumentsService) snapshotFile(ctx context.Context, id uuid.UUID, sel knowledgetypes.RefSelector, ref knowledgetypes.Ref, path string) (knowledgetypes.Snapshot, knowledgetypes.FileContent, error) {
	snap, err := s.resolveSnapshot(ctx, id, sel, ref)
	if err != nil {
		return knowledgetypes.Snapshot{}, knowledgetypes.FileContent{}, err
	}
	f, err := s.snapshotRepository.File(ctx, snap.ID, path)
	if err != nil {
		return knowledgetypes.Snapshot{}, knowledgetypes.FileContent{}, err
	}
	if f == nil {
		return knowledgetypes.Snapshot{}, knowledgetypes.FileContent{}, apperr.ErrNotFound
	}
	return snap, *f, nil
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
	sel, ref, err := s.resolveRef(ctx, kp, r)
	if err != nil {
		return knowledgetypes.Snapshot{}, knowledgetypes.FileContent{}, err
	}
	snap, f, err := s.snapshotFile(ctx, id, sel, ref, path)
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
	tagRepository        *repositories.TagRepository
	settingsService      *SettingsService
	cfg                  *config.Config
}
