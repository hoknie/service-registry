package services

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/skills/repositories"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
)

const maxFolderSkills = 5000

type ArchiveService struct {
	parts
	cards   *cardBuilder
	content *ContentService
	folders *FoldersService
}

type FolderPlan struct {
	Folder  skillstypes.Folder
	Skills  []Selected
	Skipped int
	Size    int64
}

func (s *ArchiveService) FolderPlan(ctx context.Context, p accesstypes.Principal, id uuid.UUID, withFiles bool) (FolderPlan, error) {
	f, err := s.folders.GetFolder(ctx, p, id)
	if err != nil {
		return FolderPlan{}, err
	}
	if f.Off() {
		return FolderPlan{}, apperr.Wrap(skillstypes.ConflictDisabled)
	}
	read, write, err := s.access.visible(ctx, p)
	if err != nil {
		return FolderPlan{}, err
	}
	orgs, err := s.access.setOrgs(ctx, p)
	if err != nil {
		return FolderPlan{}, apperr.Wrap(err)
	}
	q := skillstypes.ListQuery{FolderID: &id, Recursive: true, Organizations: read.Organizations, Global: read.Global,
		Writable: write.Organizations, WriteGlobal: write.Global, Active: new(true), SetOrgs: orgs, Limit: maxFolderSkills}
	rows, err := s.skills.List(ctx, repositories.Filter{ListQuery: q})
	if err != nil {
		return FolderPlan{}, apperr.Wrap(err)
	}
	items := make([]skillstypes.Skill, len(rows))
	for i, r := range rows {
		items[i] = r.Skill
	}
	slices.SortFunc(items, func(a, b skillstypes.Skill) int { return strings.Compare(a.Path, b.Path) })
	if items, err = s.cards.decorate(ctx, items, write); err != nil {
		return FolderPlan{}, err
	}
	return s.plan(ctx, f, items, withFiles)
}

func (s *ArchiveService) plan(ctx context.Context, f skillstypes.Folder, items []skillstypes.Skill, withFiles bool) (FolderPlan, error) {
	out := FolderPlan{Folder: f}
	seen := map[string]bool{}
	for _, sk := range items {
		v, err := s.cards.pick(sk, "")
		if err != nil {
			continue
		}
		if seen[sk.Name] {
			out.Skipped++
			continue
		}
		seen[sk.Name] = true
		sel := Selected{Skill: sk, Version: v}
		if withFiles {
			if sel.Files, err = s.versions.Files(ctx, v.ID); err != nil {
				return FolderPlan{}, apperr.Wrap(err)
			}
		}
		out.Size += v.Size
		out.Skills = append(out.Skills, sel)
	}
	if len(out.Skills) == 0 {
		return FolderPlan{}, apperr.Wrap(skillstypes.ConflictEmpty)
	}
	if out.Size > s.cfg.Skills.FolderArchiveMax {
		return FolderPlan{}, apperr.Wrap(skillstypes.FolderTooLarge)
	}
	return out, nil
}

func (s *ArchiveService) ReadFile(ctx context.Context, f skillstypes.File) ([]byte, error) {
	return s.content.read(ctx, f)
}

func (s *ArchiveService) CountDownloads(ctx context.Context, plan FolderPlan) error {
	ids := make([]uuid.UUID, len(plan.Skills))
	for i, sel := range plan.Skills {
		ids[i] = sel.Skill.ID
	}
	return apperr.Wrap(s.skills.CountDownloads(ctx, ids))
}
