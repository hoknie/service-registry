package services

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/skills/repositories"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
)

type SpacesService struct {
	parts
	spaces *repositories.SpaceRepository
	sets   *repositories.SetRepository
}

type Space struct {
	Organization skillstypes.Ref
	Description  string
	CanWrite     bool
}

func (s *SpacesService) OrganizationSpace(ctx context.Context, p accesstypes.Principal, org uuid.UUID) (Space, error) {
	sc, err := s.access.catalog.Access.Scope(ctx, p, org)
	if err != nil {
		return Space{}, err
	}
	if len(sc.Path) > 0 {
		return Space{}, apperr.New(apperr.NotFound)
	}
	description, err := s.spaces.Description(ctx, org)
	if err != nil {
		return Space{}, apperr.Wrap(err)
	}
	return Space{Organization: skillstypes.Ref{ID: sc.Node.ID, Slug: sc.Node.Slug, Name: sc.Node.Name}, Description: description,
		CanWrite: p.IsSuperadmin || sc.Allows(catalogtypes.PermWrite)}, nil
}

func (s *SpacesService) PutOrganizationSpace(ctx context.Context, p accesstypes.Principal, org uuid.UUID, description string) (Space, error) {
	sc, err := s.access.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, org)
	if err != nil {
		return Space{}, err
	}
	if len(sc.Path) > 0 {
		return Space{}, apperr.New(apperr.NotFound)
	}
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > 2000 {
		return Space{}, apperr.Wrap(skillstypes.InvalidSpace)
	}
	if err := s.spaces.SetDescription(ctx, org, description); err != nil {
		return Space{}, apperr.Wrap(err)
	}
	return s.OrganizationSpace(ctx, p, org)
}

type SkillSet struct {
	Mode      string
	Folders   []skillstypes.SetItem
	Skills    []skillstypes.SetItem
	CanManage bool
}

func (s *SpacesService) OrganizationSet(ctx context.Context, p accesstypes.Principal, org uuid.UUID) (SkillSet, error) {
	space, err := s.OrganizationSpace(ctx, p, org)
	if err != nil {
		return SkillSet{}, err
	}
	set, err := s.sets.Get(ctx, org)
	if err != nil {
		return SkillSet{}, apperr.Wrap(err)
	}
	folders, skills, err := s.sets.Describe(ctx, set.Folders, set.Skills)
	if err != nil {
		return SkillSet{}, apperr.Wrap(err)
	}
	return SkillSet{Mode: set.Mode, Folders: nonNilItems(folders), Skills: nonNilItems(skills), CanManage: space.CanWrite}, nil
}

func (s *SpacesService) PutOrganizationSet(ctx context.Context, p accesstypes.Principal, org uuid.UUID, mode string, folders, skills []uuid.UUID) (SkillSet, error) {
	space, err := s.OrganizationSpace(ctx, p, org)
	if err != nil {
		return SkillSet{}, err
	}
	if !space.CanWrite {
		return SkillSet{}, apperr.New(apperr.Forbidden)
	}
	set, err := skillstypes.CleanSet(mode, folders, skills)
	if err != nil {
		return SkillSet{}, apperr.Wrap(err)
	}
	gotFolders, gotSkills, err := s.sets.Describe(ctx, set.Folders, set.Skills)
	if err != nil {
		return SkillSet{}, apperr.Wrap(err)
	}
	if len(gotFolders) != len(set.Folders) || len(gotSkills) != len(set.Skills) || !allGlobal(gotFolders) || !allGlobal(gotSkills) {
		return SkillSet{}, apperr.Wrap(skillstypes.InvalidSet)
	}
	if err := s.sets.Put(ctx, org, set); err != nil {
		return SkillSet{}, apperr.Wrap(err)
	}
	return s.OrganizationSet(ctx, p, org)
}

func allGlobal(items []skillstypes.SetItem) bool {
	for _, it := range items {
		if !it.Global {
			return false
		}
	}
	return true
}

func nonNilItems(items []skillstypes.SetItem) []skillstypes.SetItem {
	if items == nil {
		return []skillstypes.SetItem{}
	}
	return items
}
