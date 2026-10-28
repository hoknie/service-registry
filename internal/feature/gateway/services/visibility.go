package services

import (
	"context"
	"slices"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/gateway/repositories"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/apperr"
)

type VisibilityService struct{ parts }

func (s *VisibilityService) ServerAccess(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (gatewaytypes.AccessList, error) {
	srv, err := readServer(ctx, s.parts, p, id)
	if err != nil {
		return gatewaytypes.AccessList{}, err
	}
	if !srv.CanWrite && !srv.CanManage {
		return gatewaytypes.AccessList{}, apperr.New(apperr.Forbidden)
	}
	groups, err := s.accessLists.Groups(ctx, id)
	if err != nil {
		return gatewaytypes.AccessList{}, apperr.Wrap(err)
	}
	users, err := s.accessLists.Users(ctx, id)
	if err != nil {
		return gatewaytypes.AccessList{}, apperr.Wrap(err)
	}
	return gatewaytypes.AccessList{Everyone: srv.Everyone, Groups: groups, Users: users, CanManage: srv.CanManage}, nil
}

func (s *VisibilityService) PutServerAccess(ctx context.Context, p accesstypes.Principal, id uuid.UUID, everyone bool,
	groups, users []uuid.UUID) (gatewaytypes.AccessList, error) {
	srv, err := readServer(ctx, s.parts, p, id)
	if err != nil {
		return gatewaytypes.AccessList{}, err
	}
	if err := s.access.canManage(ctx, p, srv.Scope()); err != nil {
		return gatewaytypes.AccessList{}, err
	}
	groups, okGroups := gatewaytypes.CleanIDs(groups, 100)
	users, okUsers := gatewaytypes.CleanIDs(users, 100)
	if !okGroups || !okUsers {
		return gatewaytypes.AccessList{}, apperr.Wrap(gatewaytypes.InvalidAccess)
	}
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		valid, err := s.accessLists.ValidSubjects(ctx, srv.OrganizationID, groups, users)
		if err != nil {
			return err
		}
		if !valid {
			return gatewaytypes.InvalidAccess
		}
		return s.accessLists.Replace(ctx, id, everyone, groups, users)
	})
	if err != nil {
		return gatewaytypes.AccessList{}, apperr.Wrap(repositories.Err(err))
	}
	return s.ServerAccess(ctx, p, id)
}

func (s *VisibilityService) organization(ctx context.Context, p accesstypes.Principal, org uuid.UUID) error {
	sc, err := s.catalog.Access.Scope(ctx, p, org)
	if err != nil {
		return err
	}
	if len(sc.Path) > 0 {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

func (s *VisibilityService) OrganizationSet(ctx context.Context, p accesstypes.Principal, org uuid.UUID) (gatewaytypes.OrgSet, error) {
	if err := s.organization(ctx, p, org); err != nil {
		return gatewaytypes.OrgSet{}, err
	}
	row, err := s.organizations.Get(ctx, org)
	if err != nil {
		return gatewaytypes.OrgSet{}, apperr.Wrap(err)
	}
	servers, err := s.organizations.Servers(ctx, org)
	if err != nil {
		return gatewaytypes.OrgSet{}, apperr.Wrap(err)
	}
	return gatewaytypes.OrgSet{Mode: row.Mode, Servers: servers,
		CanManage: s.access.manages(ctx, p, gatewaytypes.Scope{OrganizationID: &org})}, nil
}

func (s *VisibilityService) PutOrganizationSet(ctx context.Context, p accesstypes.Principal, org uuid.UUID, mode string,
	servers []uuid.UUID) (gatewaytypes.OrgSet, error) {
	if err := s.organization(ctx, p, org); err != nil {
		return gatewaytypes.OrgSet{}, err
	}
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermAccess, org); err != nil {
		return gatewaytypes.OrgSet{}, err
	}
	servers, ok := gatewaytypes.CleanIDs(servers, 200)
	if !ok || (mode != gatewaytypes.SetAll && mode != gatewaytypes.SetSelected) {
		return gatewaytypes.OrgSet{}, apperr.Wrap(gatewaytypes.InvalidSet)
	}
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		fit, err := s.organizations.Fit(ctx, org, servers)
		if err != nil {
			return err
		}
		if !fit {
			return gatewaytypes.InvalidSet
		}
		return s.organizations.ReplaceSet(ctx, org, mode, servers)
	})
	if err != nil {
		return gatewaytypes.OrgSet{}, apperr.Wrap(repositories.Err(err))
	}
	return s.OrganizationSet(ctx, p, org)
}

type SummaryColumn struct {
	Organization gatewaytypes.Ref
	Mode         string
	CanManage    bool
}

type SummaryRow struct {
	Server gatewaytypes.Server
	Access gatewaytypes.AccessList
	In     []uuid.UUID
}

type Summary struct {
	Organizations []SummaryColumn
	Servers       []SummaryRow
}

func (s *VisibilityService) Summary(ctx context.Context, p accesstypes.Principal, only *uuid.UUID) (Summary, error) {
	managed, err := s.access.managed(ctx, p)
	if err != nil {
		return Summary{}, err
	}
	if !managed.Global && len(managed.Organizations) == 0 {
		return Summary{}, apperr.New(apperr.Forbidden)
	}
	orgs := managed.Organizations
	if only != nil {
		orgs = slices.DeleteFunc(slices.Clone(orgs), func(id uuid.UUID) bool { return id != *only })
	}
	refs, err := s.folders.Organizations(ctx, orgs)
	if err != nil {
		return Summary{}, apperr.Wrap(err)
	}
	modes, err := s.organizations.Modes(ctx, orgs)
	if err != nil {
		return Summary{}, apperr.Wrap(err)
	}
	out := Summary{Organizations: make([]SummaryColumn, len(refs))}
	for i, r := range refs {
		out.Organizations[i] = SummaryColumn{Organization: r, Mode: modeOf(modes, r.ID), CanManage: true}
	}
	scope := gatewaytypes.Visible{Organizations: orgs, Global: true}
	servers, err := s.servers.List(ctx, repositories.ServerFilter{Read: scope, Write: scope})
	if err != nil {
		return Summary{}, apperr.Wrap(err)
	}
	if out.Servers, err = s.rows(ctx, servers, out.Organizations, managed); err != nil {
		return Summary{}, err
	}
	return out, nil
}

func modeOf(modes map[uuid.UUID]string, org uuid.UUID) string {
	if m, ok := modes[org]; ok {
		return m
	}
	return gatewaytypes.SetAll
}

func (s *VisibilityService) rows(ctx context.Context, servers []gatewaytypes.Server, cols []SummaryColumn,
	managed gatewaytypes.Visible) ([]SummaryRow, error) {
	ids := make([]uuid.UUID, len(servers))
	for i, srv := range servers {
		ids[i] = srv.ID
	}
	orgIDs := make([]uuid.UUID, len(cols))
	for i, c := range cols {
		orgIDs[i] = c.Organization.ID
	}
	subjects, err := s.accessLists.Summaries(ctx, ids)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	members, err := s.organizations.Memberships(ctx, orgIDs)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out := make([]SummaryRow, len(servers))
	for i, srv := range servers {
		srv.CanManage = managed.Has(srv.Scope())
		row := SummaryRow{Server: srv, Access: gatewaytypes.AccessList{Everyone: srv.Everyone, CanManage: srv.CanManage}, In: []uuid.UUID{}}
		for _, sub := range subjects {
			switch {
			case sub.ServerID != srv.ID:
			case sub.GroupID != nil:
				row.Access.Groups = append(row.Access.Groups, gatewaytypes.GroupRef{ID: *sub.GroupID, Name: sub.Label})
			case sub.UserID != nil:
				row.Access.Users = append(row.Access.Users, gatewaytypes.UserRef{ID: *sub.UserID, Email: sub.Label})
			}
		}
		for _, c := range cols {
			if inSet(srv, c, members) {
				row.In = append(row.In, c.Organization.ID)
			}
		}
		out[i] = row
	}
	return out, nil
}

func inSet(srv gatewaytypes.Server, c SummaryColumn, members []repositories.Membership) bool {
	org := c.Organization.ID
	if srv.OrganizationID != nil && *srv.OrganizationID != org {
		return false
	}
	if c.Mode == gatewaytypes.SetAll {
		return true
	}
	return slices.ContainsFunc(members, func(m repositories.Membership) bool { return m.OrganizationID == org && m.ServerID == srv.ID })
}
