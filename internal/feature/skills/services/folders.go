package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/skills/repositories"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
)

type FoldersService struct{ parts }

func folderChain(ctx context.Context, p parts, id uuid.UUID) ([]skillstypes.Folder, error) {
	chain, err := p.folders.Chain(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if len(chain) == 0 {
		return nil, apperr.New(apperr.NotFound)
	}
	return chain, nil
}

func placeOf(ctx context.Context, p parts, pr accesstypes.Principal, folder, org *uuid.UUID, action accesstypes.Action) (skillstypes.Scope, error) {
	if folder != nil {
		chain, err := folderChain(ctx, p, *folder)
		if err != nil {
			return skillstypes.Scope{}, err
		}
		scope := skillstypes.Scope{OrganizationID: chain[0].OrganizationID}
		if err := p.access.canRead(ctx, pr, scope); err != nil {
			return skillstypes.Scope{}, err
		}
		return scope, p.access.canWrite(ctx, pr, scope, action)
	}
	scope := skillstypes.Scope{OrganizationID: org}
	if org != nil {
		ok, err := p.folders.IsOrganization(ctx, *org)
		if err != nil {
			return skillstypes.Scope{}, apperr.Wrap(err)
		}
		if !ok {
			return skillstypes.Scope{}, apperr.Wrap(skillstypes.InvalidScope)
		}
		if err := p.access.canRead(ctx, pr, scope); err != nil {
			return skillstypes.Scope{}, apperr.Wrap(skillstypes.InvalidScope)
		}
	}
	return scope, p.access.canWrite(ctx, pr, scope, action)
}

func (s *FoldersService) ListFolders(ctx context.Context, p accesstypes.Principal, parent *uuid.UUID) ([]skillstypes.Folder, error) {
	if parent != nil {
		if _, err := s.GetFolder(ctx, p, *parent); err != nil {
			return nil, err
		}
	}
	read, write, err := s.access.visible(ctx, p)
	if err != nil {
		return nil, err
	}
	found, err := s.folders.List(ctx, parent, read, write)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	for i := range found {
		found[i].CanWrite = writable(write, skillstypes.Scope{OrganizationID: found[i].OrganizationID})
	}
	return found, nil
}

func (s *FoldersService) GetFolder(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (skillstypes.Folder, error) {
	chain, err := folderChain(ctx, s.parts, id)
	if err != nil {
		return skillstypes.Folder{}, err
	}
	f := chain[0]
	scope := skillstypes.Scope{OrganizationID: f.OrganizationID}
	if err := s.access.canRead(ctx, p, scope); err != nil {
		return skillstypes.Folder{}, err
	}
	f.Depth = len(chain) - 1
	for i := len(chain) - 1; i >= 1; i-- {
		f.Ancestors = append(f.Ancestors, skillstypes.Ref{ID: chain[i].ID, Slug: chain[i].Slug, Name: chain[i].Name})
	}
	f.CanWrite = s.access.writes(ctx, p, scope)
	read, write, err := s.access.visible(ctx, p)
	if err != nil {
		return skillstypes.Folder{}, err
	}
	siblings, err := s.folders.List(ctx, f.ParentID, read, write)
	if err != nil {
		return skillstypes.Folder{}, apperr.Wrap(err)
	}
	for _, sib := range siblings {
		if sib.ID == f.ID {
			f.Skills = sib.Skills
		}
	}
	return f, nil
}

type FolderInput struct {
	ParentID       *uuid.UUID
	OrganizationID *uuid.UUID
	Slug           string
	Name           string
	Description    string
}

func (s *FoldersService) CreateFolder(ctx context.Context, p accesstypes.Principal, in FolderInput) (skillstypes.Folder, error) {
	slug, name, description, err := skillstypes.CleanFolder(in.Slug, in.Name, in.Description)
	if err != nil {
		return skillstypes.Folder{}, apperr.Wrap(err)
	}
	if in.ParentID != nil {
		in.OrganizationID = nil
	}
	scope, err := placeOf(ctx, s.parts, p, in.ParentID, in.OrganizationID, accesstypes.ActionCreate)
	if err != nil {
		return skillstypes.Folder{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return skillstypes.Folder{}, apperr.Wrap(err)
	}
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.folders.Lock(ctx); err != nil {
			return err
		}
		if in.ParentID != nil {
			chain, err := s.folders.Chain(ctx, *in.ParentID)
			if err != nil {
				return err
			}
			if len(chain) >= skillstypes.MaxFolderDepth {
				return skillstypes.FolderTooDeep
			}
		}
		return s.folders.Insert(ctx, skillstypes.NewFolder{ID: id, ParentID: in.ParentID, OrganizationID: scope.OrganizationID,
			Slug: slug, Name: name, Description: description})
	})
	if err != nil {
		return skillstypes.Folder{}, apperr.Wrap(repositories.Err(err))
	}
	return s.GetFolder(ctx, p, id)
}

type FolderPatch struct {
	Slug           *string
	Name           *string
	Description    *string
	Disabled       *bool
	Move           bool
	ParentID       *uuid.UUID
	OrganizationID *uuid.UUID
}

func (s *FoldersService) UpdateFolder(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in FolderPatch) (skillstypes.Folder, error) {
	f, err := s.GetFolder(ctx, p, id)
	if err != nil {
		return skillstypes.Folder{}, err
	}
	if err := s.access.canWrite(ctx, p, skillstypes.Scope{OrganizationID: f.OrganizationID}, accesstypes.ActionUpdate); err != nil {
		return skillstypes.Folder{}, err
	}
	slug, name, description, err := skillstypes.CleanFolder(value(in.Slug, f.Slug), value(in.Name, f.Name), value(in.Description, f.Description))
	if err != nil {
		return skillstypes.Folder{}, apperr.Wrap(err)
	}
	if f.Managed() && (slug != f.Slug || name != f.Name || in.Move) {
		return skillstypes.Folder{}, apperr.Wrap(skillstypes.ConflictManaged)
	}
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.folders.Update(ctx, id, slug, name, description); err != nil {
			return err
		}
		if in.Disabled != nil {
			if err := s.folders.SetDisabled(ctx, id, *in.Disabled); err != nil {
				return err
			}
			if !*in.Disabled {
				if err := s.sources.WakeFolder(ctx, id); err != nil {
					return err
				}
			}
		}
		if !in.Move {
			return nil
		}
		return s.move(ctx, p, id, in)
	})
	if err != nil {
		return skillstypes.Folder{}, apperr.Wrap(repositories.Err(err))
	}
	return s.GetFolder(ctx, p, id)
}

func value(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	return *v
}

func (s *FoldersService) move(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in FolderPatch) error {
	if err := s.folders.Lock(ctx); err != nil {
		return err
	}
	if in.ParentID != nil {
		in.OrganizationID = nil
	}
	scope, err := placeOf(ctx, s.parts, p, in.ParentID, in.OrganizationID, accesstypes.ActionUpdate)
	if err != nil {
		return err
	}
	depth := 0
	if in.ParentID != nil {
		chain, err := s.folders.Chain(ctx, *in.ParentID)
		if err != nil {
			return err
		}
		for _, c := range chain {
			if c.ID == id {
				return skillstypes.ConflictCycle
			}
		}
		depth = len(chain)
	}
	height, err := s.folders.Height(ctx, id)
	if err != nil {
		return err
	}
	if depth+height+1 > skillstypes.MaxFolderDepth {
		return skillstypes.FolderTooDeep
	}
	return s.folders.Move(ctx, id, in.ParentID, scope.OrganizationID)
}

func (s *FoldersService) DeleteFolder(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	f, err := s.GetFolder(ctx, p, id)
	if err != nil {
		return err
	}
	if err := s.access.canWrite(ctx, p, skillstypes.Scope{OrganizationID: f.OrganizationID}, accesstypes.ActionDelete); err != nil {
		return err
	}
	if f.Managed() {
		return apperr.Wrap(skillstypes.ConflictManaged)
	}
	return apperr.Wrap(s.db.InTx(ctx, func(ctx context.Context) error {
		busy, err := s.folders.HasContent(ctx, id)
		if err != nil {
			return err
		}
		if busy {
			return skillstypes.ConflictNotEmpty
		}
		return s.folders.Delete(ctx, id)
	}))
}
