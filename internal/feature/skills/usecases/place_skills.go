package usecases

import (
	"context"
	"path"
	"strconv"

	"github.com/google/uuid"

	"svc-registry/internal/feature/skills/repositories"
	skillstypes "svc-registry/internal/feature/skills/types"
)

const maxSlugTries = 20

type PlaceSkillsUseCase struct {
	folders *repositories.FolderRepository
	skills  *repositories.SkillRepository
}

func NewPlaceSkills(folders *repositories.FolderRepository, skills *repositories.SkillRepository) *PlaceSkillsUseCase {
	return &PlaceSkillsUseCase{folders: folders, skills: skills}
}

type placement struct {
	*PlaceSkillsUseCase
	src   skillstypes.Source
	known map[string]uuid.UUID
}

func (u *PlaceSkillsUseCase) Start(ctx context.Context, src skillstypes.Source) (*placement, error) {
	known, err := u.folders.SourceFolders(ctx, src.ID)
	if err != nil {
		return nil, err
	}
	return &placement{PlaceSkillsUseCase: u, src: src, known: known}, nil
}

func (p *placement) key(root, dir string, m skillstypes.Manifest) string {
	field := ""
	if p.src.GroupField != nil {
		field = *p.src.GroupField
	}
	return skillstypes.Place(p.src.Layout, field, root, dir, m)
}

func (p *placement) Folder(ctx context.Context, key string) (uuid.UUID, error) {
	if key == "" {
		return p.src.FolderID, nil
	}
	if id, ok := p.known[key]; ok {
		return id, nil
	}
	parent := p.src.FolderID
	name := key
	if p.src.Layout == skillstypes.LayoutTree {
		if dir := path.Dir(key); dir != "." {
			id, err := p.Folder(ctx, dir)
			if err != nil {
				return uuid.Nil, err
			}
			parent = id
		}
		name = path.Base(key)
	}
	id, err := p.insert(ctx, parent, key, name)
	if err != nil {
		return uuid.Nil, err
	}
	p.known[key] = id
	return id, nil
}

func (p *placement) insert(ctx context.Context, parent uuid.UUID, key, name string) (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}
	base := skillstypes.SlugOf(name)
	for try := 1; try <= maxSlugTries; try++ {
		slug := base
		if try > 1 {
			slug = base[:min(len(base), 60)] + "-" + strconv.Itoa(try)
		}
		ok, err := p.folders.InsertSource(ctx, skillstypes.NewFolder{ID: id, ParentID: &parent, OrganizationID: p.src.OrganizationID,
			Slug: slug, Name: name}, p.src.ID, key)
		if err != nil || ok {
			return id, err
		}
	}
	return uuid.Nil, skillstypes.ConflictSlugTaken
}

func (p *placement) Move(ctx context.Context, sk skillstypes.Skill, folder uuid.UUID) error {
	if sk.FolderID != nil && *sk.FolderID == folder {
		return nil
	}
	return repositories.Err(p.skills.Move(ctx, sk.ID, &folder, p.src.OrganizationID))
}

func (p *placement) Tidy(ctx context.Context) error {
	for range skillstypes.MaxFolderDepth + 1 {
		n, err := p.folders.DropEmpty(ctx, p.src.ID)
		if err != nil || n == 0 {
			return err
		}
	}
	return nil
}
