package services

import (
	"context"

	"github.com/google/uuid"

	accessports "svc-registry/internal/feature/access/ports"
	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
)

func (s *GroupContactsService) SetContacts(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in accesstypes.GroupContacts) (accesstypes.Group, error) {
	if err := accesstypes.RequireRight(p, accesstypes.SectionGroups, accesstypes.ActionUpdate); err != nil {
		return accesstypes.Group{}, err
	}
	if err := protectGroupWrite(ctx, s.groupRepository, s.permissionRepository, s.stewards, p, accesstypes.GroupAt{ID: id}); err != nil {
		return accesstypes.Group{}, err
	}
	return s.setContacts(ctx, id, in)
}

func (s *GroupContactsService) setContacts(ctx context.Context, id uuid.UUID, in accesstypes.GroupContacts) (accesstypes.Group, error) {
	v, err := accesstypes.ValidateGroupContacts(in)
	if err != nil {
		return accesstypes.Group{}, apperr.Wrap(err)
	}
	ids := make([]uuid.UUID, len(v.Contacts))
	for i := range ids {
		ids[i] = uuid.Must(uuid.NewV7())
	}
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.groupContactRepository.SetOnCall(ctx, id, v.OnCallURL); err != nil {
			return err
		}
		return s.groupContactRepository.ReplaceContacts(ctx, id, ids, v.Contacts)
	})
	if err != nil {
		return accesstypes.Group{}, apperr.Wrap(err)
	}
	g, err := s.groupRepository.Find(ctx, id)
	if err != nil {
		return accesstypes.Group{}, apperr.Wrap(err)
	}
	if g == nil {
		return accesstypes.Group{}, apperr.New(apperr.NotFound)
	}
	return g.Group, nil
}

func (s *GroupContactsService) SetContactEditor(ctx context.Context, p accesstypes.Principal, groupID, userID uuid.UUID, on bool) error {
	if err := accesstypes.RequireRight(p, accesstypes.SectionGroups, accesstypes.ActionUpdate); err != nil {
		return err
	}
	if err := protectGroupWrite(ctx, s.groupRepository, s.permissionRepository, s.stewards, p, accesstypes.GroupAt{ID: groupID}); err != nil {
		return err
	}
	m, err := s.groupContactRepository.Membership(ctx, groupID, userID)
	if err != nil {
		return apperr.Wrap(err)
	}
	if !m.GroupExists || !m.UserExists {
		return apperr.New(apperr.NotFound)
	}
	if on && !m.Member {
		return apperr.Wrap(accesstypes.InvalidNotGroupMember)
	}
	if on {
		return apperr.Wrap(s.groupContactRepository.GrantEditor(ctx, groupID, userID))
	}
	managed, err := s.groupContactRepository.RevokeManualEditor(ctx, groupID, userID)
	if err == nil && managed {
		return apperr.Wrap(accesstypes.ConflictEditorManaged)
	}
	return apperr.Wrap(err)
}

func (s *GroupContactsService) OwnGroups(ctx context.Context, p accesstypes.Principal) ([]accesstypes.OwnGroup, error) {
	out, err := s.groupContactRepository.OwnGroups(ctx, p.UserID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	for i := range out {
		out[i].CanEditContacts = out[i].CanEditContacts || p.IsSuperadmin
	}
	return out, nil
}

func (s *GroupContactsService) SetOwnContacts(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in accesstypes.GroupContacts) (accesstypes.OwnGroup, error) {
	m, err := s.groupContactRepository.Membership(ctx, id, p.UserID)
	if err != nil {
		return accesstypes.OwnGroup{}, apperr.Wrap(err)
	}
	if !m.Member {
		return accesstypes.OwnGroup{}, apperr.New(apperr.NotFound)
	}
	if !m.Editor && !p.IsSuperadmin {
		return accesstypes.OwnGroup{}, apperr.New(apperr.Forbidden)
	}
	g, err := s.setContacts(ctx, id, in)
	return accesstypes.OwnGroup{Group: g, CanEditContacts: true}, err
}

type GroupContactsService struct {
	db                     *pgtx.DB
	permissionRepository   *repositories.GroupPermissionRepository
	groupRepository        *repositories.GroupRepository
	groupContactRepository *repositories.GroupContactRepository
	stewards               accessports.GroupStewardsPort
}
