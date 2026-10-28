package usecases

import (
	"context"
	"errors"
	"io"
	"maps"
	"slices"

	"github.com/google/uuid"

	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/feature/skills/repositories"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
)

type SyncSourceDeps struct {
	Skills       *repositories.SkillRepository
	Save         *SaveVersionUseCase
	Places       *PlaceSkillsUseCase
	Limits       skillstypes.Limits
	ArchiveLimit int64
}

type SyncSourceUseCase struct{ d SyncSourceDeps }

var errBranchMissing = errors.New("branch not found")

func NewSyncSource(d SyncSourceDeps) *SyncSourceUseCase { return &SyncSourceUseCase{d: d} }

type SyncInput struct {
	Source skillstypes.Source
	Reader knowledgeports.ReaderPort
	Author string
	Policy func(uuid.UUID) skillstypes.Effective
}

type SyncResult struct {
	Run    skillstypes.Run
	Commit *string
	Tags   map[string]string
}

type syncRun struct {
	*SyncSourceUseCase
	in       SyncInput
	run      *skillstypes.Run
	existing map[string]skillstypes.Skill
	touched  map[uuid.UUID]bool
	place    *placement
}

func (u *SyncSourceUseCase) Run(ctx context.Context, in SyncInput) SyncResult {
	run := skillstypes.Run{Status: skillstypes.RunSucceeded, Problems: []skillstypes.Problem{}}
	heads, def, err := in.Reader.Heads(ctx)
	if err != nil {
		return failed(run, err)
	}
	branch := in.Source.Branch
	if branch == "" || in.Source.Kind == string(knowledgetypes.SourceLocalDir) {
		branch = def
	}
	head, ok := heads[branch]
	if !ok {
		return failed(run, errBranchMissing)
	}
	run.Commit = &head
	tags, pattern, err := u.newTags(ctx, in)
	if err != nil {
		return failed(run, err)
	}
	if in.Source.LastCommit != nil && *in.Source.LastCommit == head && len(tags) == 0 {
		run.Status = skillstypes.RunUnchanged
		return SyncResult{Run: run}
	}
	s := &syncRun{SyncSourceUseCase: u, in: in, run: &run, touched: map[uuid.UUID]bool{}}
	if err := s.load(ctx); err != nil {
		return failed(run, err)
	}
	if err := s.branch(ctx, branch, head); err != nil {
		return failed(run, err)
	}
	done := maps.Clone(in.Source.LastTags)
	if done == nil {
		done = map[string]string{}
	}
	if err := s.tags(ctx, tags, pattern, done); err != nil {
		return failed(run, err)
	}
	run.Updated = len(s.touched)
	return SyncResult{Run: run, Commit: &head, Tags: done}
}

func failed(run skillstypes.Run, err error) SyncResult {
	run.Status = skillstypes.RunFailed
	run.Error = new(skillstypes.FailureCode(err, errors.Is(err, errBranchMissing)))
	if errors.Is(err, knowledgetypes.ErrSnapshotTooLarge) {
		run.Error = new(skillstypes.FailArchiveTooLarge)
	}
	run.Created, run.Updated, run.Removed, run.Skipped = 0, 0, 0, 0
	return SyncResult{Run: run}
}

func (u *SyncSourceUseCase) newTags(ctx context.Context, in SyncInput) (map[string]string, skillstypes.TagPattern, error) {
	if in.Source.Versions != skillstypes.VersionsTags {
		return nil, skillstypes.TagPattern{}, nil
	}
	pattern, err := skillstypes.ParseTagPattern(in.Source.TagPattern)
	if err != nil {
		return nil, pattern, err
	}
	all, err := in.Reader.Tags(ctx)
	if err != nil {
		return nil, pattern, err
	}
	return skillstypes.NewTags(all, in.Source.LastTags, pattern), pattern, nil
}

func (s *syncRun) load(ctx context.Context) error {
	rows, err := s.d.Skills.BySource(ctx, s.in.Source.ID)
	if err != nil {
		return err
	}
	s.existing = map[string]skillstypes.Skill{}
	for _, r := range rows {
		if r.Skill.SourcePath != nil {
			s.existing[*r.Skill.SourcePath] = r.Skill
		}
	}
	s.place, err = s.d.Places.Start(ctx, s.in.Source)
	return err
}

func (s *syncRun) tree(ctx context.Context, branch, commit string) (tree, bool, error) {
	return readTree(ctx, s.in.Reader, branch, commit, s.d.ArchiveLimit)
}

func (s *syncRun) content(t tree, dir string) (skillstypes.Content, error) {
	var entries []skillstypes.Entry
	for _, p := range skillstypes.FilesUnder(t.paths, dir) {
		if size := t.size[p]; size > s.d.Limits.FileBytes && s.d.Limits.FileBytes > 0 {
			return skillstypes.Content{}, skillstypes.ContentTooLarge
		}
		entries = append(entries, skillstypes.Entry{Path: skillstypes.RelativePath(p, dir), Open: func() (io.ReadCloser, error) {
			return t.open(p, s.d.Limits.FileBytes+1)
		}})
	}
	return skillstypes.Build(entries, s.d.Limits)
}

func (s *syncRun) problem(err error, dir string) error {
	code := skillstypes.ProblemOf(err)
	if code == skillstypes.FailStorage || code == "" {
		return err
	}
	s.run.Skipped++
	s.run.Problem(code, dir, apperr.From(err).Message)
	return nil
}

func (s *syncRun) branch(ctx context.Context, branch, head string) error {
	t, truncated, err := s.tree(ctx, branch, head)
	if err != nil {
		return err
	}
	defer t.release()
	roots := s.in.Source.Roots
	for _, missing := range skillstypes.MissingRoots(t.paths, roots) {
		s.run.Problem(skillstypes.ProblemRootMissing, missing, "")
	}
	dirs := skillstypes.FindSkillDirsIn(t.paths, roots)
	for _, dir := range dirs {
		c, err := s.content(t, dir)
		if err != nil {
			if err := s.problem(err, dir); err != nil {
				return err
			}
			continue
		}
		if err := s.save(ctx, dir, c, head, nil, s.in.Source.Versions != skillstypes.VersionsTags); err != nil {
			return err
		}
	}
	if truncated {
		s.run.Problem(skillstypes.ProblemTreeTruncated, roots[0], "")
		return nil
	}
	for p, sk := range s.existing {
		if !slices.Contains(dirs, p) {
			if err := s.d.Skills.Delete(ctx, sk.ID); err != nil {
				return err
			}
			s.run.Removed++
		}
	}
	return s.place.Tidy(ctx)
}

func (s *syncRun) save(ctx context.Context, dir string, c skillstypes.Content, commit string, tag *skillstypes.Tagged, version bool) error {
	sk, ok := s.existing[dir]
	if ok && sk.Name != c.Manifest.Name {
		return s.problem(skillstypes.ConflictName, dir)
	}
	folder, err := s.place.Folder(ctx, s.place.key(skillstypes.RootOf(dir, s.in.Source.Roots), dir, c.Manifest))
	if err != nil {
		return s.problem(err, dir)
	}
	if ok && tag == nil {
		if err := s.place.Move(ctx, sk, folder); err != nil {
			return s.problem(err, dir)
		}
	}
	in := SaveInput{SkillID: sk.ID, Content: c, Origin: skillstypes.OriginGit, Commit: &commit, Author: s.in.Author,
		ManagedTags: true, Policy: s.in.Policy}
	if tag != nil {
		in.FormVersion, in.Tag = tag.Version, &tag.Name
	}
	if !ok {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		in.New = &repositories.NewSkill{ID: id, FolderID: &folder, OrganizationID: s.in.Source.OrganizationID,
			Slug: c.Manifest.Name, Status: s.in.Source.NewStatus, Tags: c.Manifest.Tags, SourceID: &s.in.Source.ID,
			SourcePath: &dir}
	}
	if !version && ok {
		return nil
	}
	if !version {
		return s.create(ctx, dir, in)
	}
	res, err := s.d.Save.Run(ctx, in)
	if err != nil {
		return s.problem(err, dir)
	}
	return s.count(dir, ok, res)
}

func (s *syncRun) create(ctx context.Context, dir string, in SaveInput) error {
	in.New.Manifest = in.Content.Manifest
	if err := repositories.Err(s.d.Skills.Insert(ctx, *in.New)); err != nil {
		return s.problem(err, dir)
	}
	s.existing[dir] = skillstypes.Skill{ID: in.New.ID, Name: in.Content.Manifest.Name}
	s.run.Created++
	return nil
}

func (s *syncRun) count(dir string, existed bool, res SaveResult) error {
	if !existed {
		s.existing[dir] = skillstypes.Skill{ID: res.Version.SkillID, Name: res.Version.Manifest.Name}
		s.run.Created++
		return nil
	}
	if res.Created {
		s.touched[res.Version.SkillID] = true
	}
	return nil
}

func (s *syncRun) tags(ctx context.Context, found map[string]string, pattern skillstypes.TagPattern, done map[string]string) error {
	ordered := skillstypes.OrderTags(found, pattern)
	if len(ordered) > skillstypes.MaxTagsPerRun {
		ordered = ordered[:skillstypes.MaxTagsPerRun]
		s.run.Problem(skillstypes.ProblemTagsDeferred, "", "")
	}
	for _, tag := range ordered {
		t, _, err := s.tree(ctx, tag.Name, tag.Commit)
		if err != nil {
			return err
		}
		defer t.release()
		for _, dir := range skillstypes.FindSkillDirsIn(t.paths, s.in.Source.Roots) {
			if _, known := s.existing[dir]; !known {
				continue
			}
			c, err := s.content(t, dir)
			if err != nil {
				if err := s.problem(err, dir); err != nil {
					return err
				}
				continue
			}
			if tag.Skill != "" && tag.Skill != c.Manifest.Name {
				continue
			}
			if c.Manifest.Version != "" && c.Manifest.Version != tag.Version {
				s.run.Problem(skillstypes.ProblemVersionMismatch, dir, tag.Name)
			}
			if err := s.save(ctx, dir, c, tag.Commit, &tag, true); err != nil {
				return err
			}
		}
		done[tag.Name] = tag.Commit
	}
	return nil
}
