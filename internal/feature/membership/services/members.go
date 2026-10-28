package services

import (
	"context"
	"encoding/base64"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/membership/repositories"
	membershiptypes "svc-registry/internal/feature/membership/types"
	"svc-registry/internal/feature/membership/usecases"
	"svc-registry/internal/platform/apperr"
)

const (
	defaultPage        = 50
	maxPage            = 200
	candidatesLimit    = 20
	candidatesMinRunes = 3
)

type MembersService struct {
	parts
	remove *usecases.RemoveMemberUseCase
}

type MemberQuery struct {
	Q      string
	Limit  int
	Cursor string
}

type AddMember struct {
	UserID *uuid.UUID
	Email  *string
	Role   *string
}

func (p parts) organization(ctx context.Context, pr accesstypes.Principal, id uuid.UUID, perm catalogtypes.Permission) (catalogservices.Scoped, error) {
	scoped, err := p.catalog.Access.Authorize(ctx, pr, perm, id)
	if err != nil {
		return scoped, err
	}
	if scoped.Node.ParentID != nil || scoped.Node.Kind != catalogtypes.KindOrganization {
		return scoped, apperr.Wrap(membershiptypes.NotOrganization)
	}
	return scoped, nil
}

func (s *MembersService) ListMembers(ctx context.Context, p accesstypes.Principal, id uuid.UUID, q MemberQuery) (membershiptypes.MemberPage, error) {
	if _, err := s.organization(ctx, p, id, catalogtypes.PermRead); err != nil {
		return membershiptypes.MemberPage{}, err
	}
	limit := q.Limit
	if limit == 0 {
		limit = defaultPage
	}
	if limit < 1 || limit > maxPage {
		return membershiptypes.MemberPage{}, apperr.Wrap(membershiptypes.InvalidPagination)
	}
	f := repositories.MemberFilter{Organization: id, Limit: limit + 1}
	if q.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(q.Cursor)
		name, rawID, ok := strings.Cut(string(raw), "\x01")
		after, idErr := uuid.Parse(rawID)
		if err != nil || !ok || idErr != nil {
			return membershiptypes.MemberPage{}, apperr.Wrap(membershiptypes.InvalidPagination)
		}
		f.AfterName, f.AfterID = &name, &after
	}
	if text := strings.TrimSpace(q.Q); text != "" {
		pattern := catalogtypes.LikeContains(strings.ToLower(text))
		f.Pattern = &pattern
	}
	items, err := s.members.List(ctx, f)
	if err != nil {
		return membershiptypes.MemberPage{}, apperr.Wrap(err)
	}
	page := membershiptypes.MemberPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		next := base64.RawURLEncoding.EncodeToString([]byte(page.Items[limit-1].Cursor))
		page.NextCursor = &next
	}
	if err := s.withRoles(ctx, id, page.Items); err != nil {
		return membershiptypes.MemberPage{}, err
	}
	_, denied := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermAccess, id)
	page.CanManage = denied == nil
	return page, nil
}

func (s *MembersService) withRoles(ctx context.Context, org uuid.UUID, items []membershiptypes.Member) error {
	ids := make([]uuid.UUID, len(items))
	for i, m := range items {
		ids[i] = m.User.ID
	}
	roles, err := s.catalog.OrgRoles.Roles(ctx, org, ids)
	if err != nil {
		return err
	}
	for i := range items {
		if r, ok := roles[items[i].User.ID]; ok {
			role := membershiptypes.Role(r)
			items[i].Role = &role
		}
	}
	return nil
}

func (s *MembersService) GroupsWithAccess(ctx context.Context, p accesstypes.Principal, id uuid.UUID) ([]membershiptypes.GroupAccess, error) {
	if _, err := s.organization(ctx, p, id, catalogtypes.PermRead); err != nil {
		return nil, err
	}
	groups, err := s.catalog.OrgRoles.GlobalGroups(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]membershiptypes.GroupAccess, len(groups))
	for i, g := range groups {
		out[i] = membershiptypes.GroupAccess{ID: g.ID, Name: g.Name, Role: membershiptypes.Role(g.Role), Users: g.Users}
	}
	return out, nil
}

func (s *MembersService) Candidates(ctx context.Context, p accesstypes.Principal, id uuid.UUID, q string) ([]membershiptypes.Person, error) {
	if _, err := s.organization(ctx, p, id, catalogtypes.PermAccess); err != nil {
		return nil, err
	}
	q = strings.TrimSpace(q)
	if !p.IsSuperadmin && utf8.RuneCountInString(q) < candidatesMinRunes {
		return []membershiptypes.Person{}, nil
	}
	out, err := s.members.Candidates(ctx, id, catalogtypes.LikeContains(strings.ToLower(q)), candidatesLimit)
	return out, apperr.Wrap(err)
}

func (s *MembersService) AddMember(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in AddMember) (membershiptypes.Member, bool, error) {
	if _, err := s.organization(ctx, p, id, catalogtypes.PermAccess); err != nil {
		return membershiptypes.Member{}, false, err
	}
	role := string(membershiptypes.Viewer)
	if in.Role != nil {
		role = *in.Role
	}
	if _, ok := membershiptypes.ParseRole(role); !ok || (in.UserID == nil) == (in.Email == nil) {
		return membershiptypes.Member{}, false, apperr.Wrap(membershiptypes.InvalidMember)
	}
	person, err := s.person(ctx, in)
	if err != nil {
		return membershiptypes.Member{}, false, err
	}
	created := false
	err = s.keepAdmin.Run(ctx, id, func(ctx context.Context) error {
		if created, err = s.members.Add(ctx, id, person.ID, membershiptypes.SourceManual, &p.UserID); err != nil {
			return err
		}
		return s.catalog.OrgRoles.SetRole(ctx, id, person.ID, &role)
	})
	if err != nil {
		return membershiptypes.Member{}, false, apperr.Wrap(repositories.Err(err))
	}
	m, err := s.member(ctx, id, person.ID)
	return m, created, err
}

func (s *MembersService) person(ctx context.Context, in AddMember) (*membershiptypes.Person, error) {
	var found *membershiptypes.Person
	var err error
	if in.UserID != nil {
		found, err = s.members.User(ctx, *in.UserID)
	} else {
		email, ok := membershiptypes.NormalizeEmail(*in.Email)
		if !ok {
			return nil, apperr.Wrap(membershiptypes.InvalidMember)
		}
		found, err = s.members.UserByEmail(ctx, email)
	}
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if found == nil {
		return nil, apperr.New(apperr.NotFound)
	}
	return found, nil
}

func (s *MembersService) member(ctx context.Context, org, user uuid.UUID) (membershiptypes.Member, error) {
	items, err := s.members.List(ctx, repositories.MemberFilter{Organization: org, UserID: &user, Limit: 1})
	if err != nil {
		return membershiptypes.Member{}, apperr.Wrap(err)
	}
	if len(items) == 0 {
		return membershiptypes.Member{}, apperr.New(apperr.NotFound)
	}
	if err := s.withRoles(ctx, org, items); err != nil {
		return membershiptypes.Member{}, err
	}
	return items[0], nil
}

func (s *MembersService) SetMemberRole(ctx context.Context, p accesstypes.Principal, id, user uuid.UUID, role *string) (membershiptypes.Member, error) {
	if _, err := s.organization(ctx, p, id, catalogtypes.PermAccess); err != nil {
		return membershiptypes.Member{}, err
	}
	if role != nil {
		if _, ok := membershiptypes.ParseRole(*role); !ok {
			return membershiptypes.Member{}, apperr.Wrap(membershiptypes.InvalidMember)
		}
	}
	member, err := s.members.IsMember(ctx, id, user)
	if err != nil {
		return membershiptypes.Member{}, apperr.Wrap(err)
	}
	if !member {
		return membershiptypes.Member{}, apperr.New(apperr.NotFound)
	}
	err = s.keepAdmin.Run(ctx, id, func(ctx context.Context) error { return s.catalog.OrgRoles.SetRole(ctx, id, user, role) })
	if err != nil {
		return membershiptypes.Member{}, apperr.Wrap(err)
	}
	return s.member(ctx, id, user)
}

func (s *MembersService) RemoveMember(ctx context.Context, p accesstypes.Principal, id, user uuid.UUID) error {
	if _, err := s.organization(ctx, p, id, catalogtypes.PermAccess); err != nil {
		return err
	}
	return apperr.Wrap(s.remove.Run(ctx, id, user))
}
