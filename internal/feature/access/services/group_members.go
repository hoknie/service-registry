package services

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
)

const (
	candidatesLimit         = 20
	scopedCandidatesLimit   = 10
	scopedCandidatesMinRune = 3
)

func (s *GroupsService) AddMember(ctx context.Context, p accesstypes.Principal, at accesstypes.GroupAt, userID uuid.UUID) error {
	g, err := s.mayManageMembers(ctx, p, at)
	if err != nil {
		return err
	}
	if !p.IsSuperadmin && userID == p.UserID {
		admin, err := s.organizationAdmin(ctx, p, g.Organization)
		if err != nil {
			return err
		}
		if !admin {
			return apperr.New(apperr.Forbidden)
		}
	}
	return apperr.Wrap(s.groupRepository.AddMember(ctx, uuid.Must(uuid.NewV7()), at.ID, userID))
}

func (s *GroupsService) RemoveMember(ctx context.Context, p accesstypes.Principal, at accesstypes.GroupAt, userID uuid.UUID) error {
	if _, err := s.mayManageMembers(ctx, p, at); err != nil {
		return err
	}
	return apperr.Wrap(s.groupRepository.RemoveMember(ctx, at.ID, userID))
}

func (s *GroupsService) ManagedGroups(ctx context.Context, p accesstypes.Principal) ([]accesstypes.ManagedGroup, error) {
	if s.stewards == nil {
		return nil, nil
	}
	bound, err := s.stewards.GroupNodes(ctx, nil)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	var all []uuid.UUID
	for _, nodes := range bound {
		all = append(all, nodes...)
	}
	owned, err := s.stewards.OwnedAmong(ctx, p.UserID, all)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	ids, refs, err := s.stewarded(ctx, bound, owned)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	groups, err := s.groupRepository.ByIDs(ctx, ids)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out := make([]accesstypes.ManagedGroup, 0, len(groups))
	for _, g := range groups {
		m := accesstypes.ManagedGroup{Group: g}
		for _, n := range bound[g.ID] {
			m.Nodes = append(m.Nodes, refs[n])
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *GroupsService) stewarded(ctx context.Context, bound map[uuid.UUID][]uuid.UUID, owned []uuid.UUID) ([]uuid.UUID, map[uuid.UUID]accesstypes.NodeRef, error) {
	mine := map[uuid.UUID]bool{}
	for _, n := range owned {
		mine[n] = true
	}
	var ids, nodes []uuid.UUID
	for group, list := range bound {
		if !allIn(list, mine) {
			continue
		}
		protected, err := s.permissionRepository.GroupHasAny(ctx, group)
		if err != nil {
			return nil, nil, apperr.Wrap(err)
		}
		if !protected {
			ids = append(ids, group)
			nodes = append(nodes, list...)
		}
	}
	if len(ids) == 0 {
		return nil, nil, nil
	}
	refs, err := nodeRefs(ctx, s.roots, [][]uuid.UUID{nodes})
	if err != nil {
		return nil, nil, err
	}
	byID := map[uuid.UUID]accesstypes.NodeRef{}
	for _, r := range refs[0] {
		byID[r.ID] = r
	}
	return ids, byID, nil
}

func (s *GroupsService) ManagedGroup(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (accesstypes.GroupDetails, error) {
	g, err := s.groupRepository.Find(ctx, id)
	if err != nil {
		return accesstypes.GroupDetails{}, apperr.Wrap(err)
	}
	if g == nil {
		return accesstypes.GroupDetails{}, apperr.New(apperr.NotFound)
	}
	lock, can, err := s.membership(ctx, p, g.Group)
	if err != nil {
		return accesstypes.GroupDetails{}, err
	}
	if !can {
		return accesstypes.GroupDetails{}, apperr.New(apperr.NotFound)
	}
	g.MembersLock, g.CanManageMembers = lock, true
	return *g, nil
}

func (s *GroupsService) MemberCandidates(ctx context.Context, p accesstypes.Principal, at accesstypes.GroupAt, q string) ([]accesstypes.MemberCandidate, error) {
	g, err := s.memberGroup(ctx, p, at)
	if err != nil {
		return nil, err
	}
	if _, can, err := s.membership(ctx, p, g.Group); err != nil || !can {
		if err != nil {
			return nil, err
		}
		return nil, apperr.New(apperr.NotFound)
	}
	limit := int64(candidatesLimit)
	if g.Group.Organization != nil && !p.IsSuperadmin && !p.Rights.Has(accesstypes.SectionUsers, accesstypes.ActionRead) {
		if utf8.RuneCountInString(strings.TrimSpace(q)) < scopedCandidatesMinRune {
			return []accesstypes.MemberCandidate{}, nil
		}
		limit = scopedCandidatesLimit
	}
	var exclude *uuid.UUID
	if !p.IsSuperadmin {
		exclude = &p.UserID
	}
	out, err := s.groupRepository.Candidates(ctx, at.ID, accesstypes.LikeContains(q), exclude, limit)
	return out, apperr.Wrap(err)
}

func (s *GroupsService) memberGroup(ctx context.Context, p accesstypes.Principal, at accesstypes.GroupAt) (*accesstypes.GroupDetails, error) {
	var org *accesstypes.OrganizationAccess
	if at.Organization != nil {
		access, err := s.organizationAccess(ctx, p, *at.Organization)
		if err != nil {
			return nil, err
		}
		org = &access
	}
	g, err := s.groupRepository.Find(ctx, at.ID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if g == nil && org == nil {
		return &accesstypes.GroupDetails{Group: accesstypes.Group{ID: at.ID}}, nil
	}
	if g == nil || (org != nil && (g.Group.Organization == nil || g.Group.Organization.ID != org.Org.ID)) {
		return nil, apperr.New(apperr.NotFound)
	}
	return g, nil
}

func (s *GroupsService) mayManageMembers(ctx context.Context, p accesstypes.Principal, at accesstypes.GroupAt) (accesstypes.Group, error) {
	g, err := s.memberGroup(ctx, p, at)
	if err != nil {
		return accesstypes.Group{}, err
	}
	_, can, err := s.membership(ctx, p, g.Group)
	if err != nil {
		return accesstypes.Group{}, err
	}
	if !can {
		return accesstypes.Group{}, apperr.New(apperr.Forbidden)
	}
	return g.Group, nil
}

func (s *GroupsService) membership(ctx context.Context, p accesstypes.Principal, g accesstypes.Group) (accesstypes.MembersLock, bool, error) {
	protected, err := s.permissionRepository.GroupHasAny(ctx, g.ID)
	if err != nil {
		return accesstypes.LockNone, false, apperr.Wrap(err)
	}
	if protected {
		return accesstypes.LockRights, p.IsSuperadmin, nil
	}
	nodes, err := s.boundNodes(ctx, g.ID)
	if err != nil {
		return accesstypes.LockNone, false, err
	}
	lock := accesstypes.LockNone
	if len(nodes) > 0 {
		lock = accesstypes.LockBindings
	}
	if p.IsSuperadmin {
		return lock, true, nil
	}
	admin, err := s.organizationAdmin(ctx, p, g.Organization)
	if err != nil || admin {
		return lock, admin, err
	}
	if len(nodes) == 0 {
		return lock, accesstypes.RequireRight(p, accesstypes.SectionGroups, accesstypes.ActionUpdate) == nil, nil
	}
	owned, err := s.stewards.OwnedAmong(ctx, p.UserID, nodes)
	if err != nil {
		return lock, false, apperr.Wrap(err)
	}
	return lock, len(owned) == len(nodes), nil
}

func (s *GroupsService) boundNodes(ctx context.Context, groupID uuid.UUID) ([]uuid.UUID, error) {
	if s.stewards == nil {
		return nil, nil
	}
	bound, err := s.stewards.GroupNodes(ctx, []uuid.UUID{groupID})
	return bound[groupID], apperr.Wrap(err)
}
