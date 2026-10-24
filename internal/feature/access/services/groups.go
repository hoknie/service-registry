package services

import (
	"context"
	"errors"

	"github.com/google/uuid"

	accessports "svc-registry/internal/feature/access/ports"
	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
)

func (s *GroupsService) CreateGroup(ctx context.Context, p accesstypes.Principal, in accesstypes.GroupName) (accesstypes.Group, error) {
	if in.Organization == nil {
		if err := accesstypes.RequireRight(p, accesstypes.SectionGroups, accesstypes.ActionCreate); err != nil {
			return accesstypes.Group{}, err
		}
	} else if _, err := s.manageOrganization(ctx, p, *in.Organization, accesstypes.ActionCreate); err != nil {
		return accesstypes.Group{}, err
	}
	name, err := accesstypes.ValidateGroupName(in.Name)
	if err != nil {
		return accesstypes.Group{}, apperr.Wrap(err)
	}
	g, err := s.groupRepository.Insert(ctx, accesstypes.NewGroup{ID: uuid.Must(uuid.NewV7()), Name: name, Organization: in.Organization})
	return g, apperr.Wrap(err)
}

func (s *GroupsService) ListGroups(ctx context.Context, p accesstypes.Principal, q accesstypes.GroupQuery) (accesstypes.GroupList, error) {
	var out accesstypes.GroupList
	if q.Organization == nil {
		if err := accesstypes.RequireRight(p, accesstypes.SectionGroups, accesstypes.ActionRead); err != nil {
			return out, err
		}
	} else {
		org, err := s.organizationAccess(ctx, p, *q.Organization)
		if err != nil {
			return out, err
		}
		out.Organization, out.CanManage = &org.Org, mayManageOrganization(p, org, accesstypes.ActionCreate)
	}
	page, err := accesstypes.ValidatePage(q.Page)
	if err != nil {
		return out, apperr.Wrap(err)
	}
	var orgID *uuid.UUID
	if out.Organization != nil {
		orgID = &out.Organization.ID
	}
	out.Page, err = s.groupRepository.List(ctx, page, orgID)
	return out, apperr.Wrap(err)
}

func (s *GroupsService) UserGroups(ctx context.Context, p accesstypes.Principal, userID uuid.UUID) ([]accesstypes.Group, error) {
	if err := accesstypes.RequireRight(p, accesstypes.SectionUsers, accesstypes.ActionRead); err != nil {
		return nil, err
	}
	user, err := s.userRepository.Find(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if user == nil {
		return nil, apperr.New(apperr.NotFound)
	}
	groups, err := s.groupRepository.ForUser(ctx, userID)
	return groups, apperr.Wrap(err)
}

func (s *GroupsService) GetGroup(ctx context.Context, p accesstypes.Principal, at accesstypes.GroupAt) (accesstypes.GroupDetails, error) {
	g, org, err := s.locate(ctx, p, at, accesstypes.ActionRead)
	if err != nil {
		return accesstypes.GroupDetails{}, err
	}
	if g.Permissions, err = s.permissionRepository.ForGroup(ctx, at.ID); err != nil {
		return accesstypes.GroupDetails{}, apperr.Wrap(err)
	}
	if g.MembersLock, g.CanManageMembers, err = s.membership(ctx, p, g.Group); err != nil {
		return accesstypes.GroupDetails{}, err
	}
	if org != nil && !g.CanManageMembers && !org.Admin && !p.IsSuperadmin && !p.Rights.Has(accesstypes.SectionGroups, accesstypes.ActionRead) {
		g.Members, g.MembersHidden = []accesstypes.Member{}, true
	}
	return *g, nil
}

func (s *GroupsService) RenameGroup(ctx context.Context, p accesstypes.Principal, at accesstypes.GroupAt, in accesstypes.GroupName) (accesstypes.Group, error) {
	if _, _, err := s.locate(ctx, p, at, accesstypes.ActionUpdate); err != nil {
		return accesstypes.Group{}, err
	}
	if err := s.unprotected(ctx, p, at.ID); err != nil {
		return accesstypes.Group{}, err
	}
	name, err := accesstypes.ValidateGroupName(in.Name)
	if err != nil {
		return accesstypes.Group{}, apperr.Wrap(err)
	}
	g, err := s.groupRepository.Rename(ctx, at.ID, name)
	return g, apperr.Wrap(err)
}

func (s *GroupsService) DeleteGroup(ctx context.Context, p accesstypes.Principal, at accesstypes.GroupAt) error {
	if _, _, err := s.locate(ctx, p, at, accesstypes.ActionDelete); err != nil {
		return err
	}
	if err := s.unprotected(ctx, p, at.ID); err != nil {
		return err
	}
	return apperr.Wrap(s.groupRepository.Delete(ctx, at.ID))
}

func (s *GroupsService) SetGroupPermissions(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in map[string][]string) (accesstypes.Rights, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return nil, err
	}
	rights, err := accesstypes.ValidateRights(in)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	g, err := s.groupRepository.Find(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if g != nil && g.Group.Organization != nil {
		return nil, apperr.Wrap(accesstypes.InvalidOrgGroupRights)
	}
	if err := s.permissionRepository.Replace(ctx, id, rights); err != nil {
		return nil, apperr.Wrap(err)
	}
	return rights, nil
}

func (s *GroupsService) unprotected(ctx context.Context, p accesstypes.Principal, groupID uuid.UUID) error {
	return unprotectedGroup(ctx, s.permissionRepository, p, groupID)
}

func unprotectedGroup(ctx context.Context, r *repositories.GroupPermissionRepository, p accesstypes.Principal, groupID uuid.UUID) error {
	if p.IsSuperadmin {
		return nil
	}
	protected, err := r.GroupHasAny(ctx, groupID)
	if err != nil {
		return apperr.Wrap(err)
	}
	if protected {
		return apperr.New(apperr.Forbidden)
	}
	return nil
}

func (s *GroupsService) locate(ctx context.Context, p accesstypes.Principal, at accesstypes.GroupAt, action accesstypes.Action) (*accesstypes.GroupDetails, *accesstypes.OrganizationAccess, error) {
	var org *accesstypes.OrganizationAccess
	if at.Organization == nil {
		if err := accesstypes.RequireRight(p, accesstypes.SectionGroups, action); err != nil {
			return nil, nil, err
		}
	} else {
		access, err := s.organizationAccess(ctx, p, *at.Organization)
		if err != nil {
			return nil, nil, err
		}
		if action != accesstypes.ActionRead {
			if !access.TopLevel {
				return nil, nil, apperr.Wrap(accesstypes.InvalidOrgTopLevel)
			}
			if !mayManageOrganization(p, access, action) {
				return nil, nil, apperr.New(apperr.Forbidden)
			}
		}
		org = &access
	}
	g, err := s.groupRepository.Find(ctx, at.ID)
	if err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	if g == nil || (org != nil && (g.Group.Organization == nil || g.Group.Organization.ID != org.Org.ID)) {
		return nil, nil, apperr.New(apperr.NotFound)
	}
	return g, org, nil
}

func (s *GroupsService) organizationAccess(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID) (accesstypes.OrganizationAccess, error) {
	if s.organizations == nil {
		return accesstypes.OrganizationAccess{}, apperr.New(apperr.NotFound)
	}
	org, err := s.organizations.OrganizationAccess(ctx, p, nodeID)
	return org, apperr.Wrap(err)
}

func (s *GroupsService) manageOrganization(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID, action accesstypes.Action) (accesstypes.OrganizationAccess, error) {
	org, err := s.organizationAccess(ctx, p, nodeID)
	if err != nil {
		return org, err
	}
	if !org.TopLevel {
		return org, apperr.Wrap(accesstypes.InvalidOrgTopLevel)
	}
	if !mayManageOrganization(p, org, action) {
		return org, apperr.New(apperr.Forbidden)
	}
	return org, nil
}

func mayManageOrganization(p accesstypes.Principal, org accesstypes.OrganizationAccess, action accesstypes.Action) bool {
	return p.IsSuperadmin || org.Admin || accesstypes.RequireRight(p, accesstypes.SectionGroups, action) == nil
}

func (s *GroupsService) organizationAdmin(ctx context.Context, p accesstypes.Principal, org *accesstypes.NodeRef) (bool, error) {
	if org == nil || s.organizations == nil {
		return false, nil
	}
	access, err := s.organizations.OrganizationAccess(ctx, p, org.ID)
	if e, ok := errors.AsType[*apperr.Error](apperr.Wrap(err)); ok && (e.Kind == apperr.NotFound || e.Kind == apperr.Forbidden) {
		return false, nil
	}
	if err != nil {
		return false, apperr.Wrap(err)
	}
	return access.Admin, nil
}

type GroupsService struct {
	permissionRepository *repositories.GroupPermissionRepository
	groupRepository      *repositories.GroupRepository
	userRepository       *repositories.UserRepository
	stewards             accessports.GroupStewardsPort
	roots                accessports.NodeRootsPort
	organizations        accessports.OrganizationsPort
}
