package repositories

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
)

func status(raw string) (accesstypes.UserStatus, error) {
	s, ok := accesstypes.ParseStatus(raw)
	if !ok {
		return "", apperr.InternalDetail("unknown status " + raw)
	}
	return s, nil
}

func scanUser(row pgx.CollectableRow) (accesstypes.User, error) {
	return scanUserWith(row, nil, nil)
}

func scanUserWith(row pgx.Row, leading, trailing []any) (accesstypes.User, error) {
	var u accesstypes.User
	var st string
	dest := append(leading, &u.ID, &u.Email, &u.DisplayName, &st, &u.IsSuperadmin, &u.IsService, &u.HasPassword, &u.CreatedAt, &u.UpdatedAt)
	if err := row.Scan(append(dest, trailing...)...); err != nil {
		return accesstypes.User{}, err
	}
	var err error
	u.Status, err = status(st)
	return u, err
}

func parseScopes(raw []string) (accesstypes.Scopes, error) {
	out := make(accesstypes.Scopes, 0, len(raw))
	for _, r := range raw {
		sc, ok := accesstypes.ParseScope(r)
		if !ok {
			return nil, apperr.InternalDetail("unknown scope " + r)
		}
		out = append(out, sc)
	}
	return out.Canonical(), nil
}

func scanToken(row pgx.CollectableRow) (accesstypes.Token, error) {
	var t accesstypes.Token
	var scopes []string
	var st string
	err := row.Scan(&t.ID, &t.UserID, &t.UserEmail, &t.Name, &t.Prefix, &scopes,
		&t.CreatedAt, &t.ExpiresAt, &t.LastUsedAt, &t.RevokedAt, &st, &t.NodeIDs, &t.CatalogAdmin)
	if err != nil {
		return accesstypes.Token{}, err
	}
	if t.Scopes, err = parseScopes(scopes); err != nil {
		return accesstypes.Token{}, err
	}
	var ok bool
	if t.Status, ok = accesstypes.ParseTokenStatus(st); !ok {
		return accesstypes.Token{}, apperr.InternalDetail("unknown token status " + st)
	}
	return t, nil
}

func scanGroup(row pgx.CollectableRow) (accesstypes.Group, error) {
	var g accesstypes.Group
	var orgID *uuid.UUID
	var orgName *string
	err := row.Scan(&g.ID, &g.Name, &g.MemberCount, &g.OnCallURL, &g.CreatedAt, &g.UpdatedAt, &orgID, &orgName)
	g.Contacts = []accesstypes.Contact{}
	g.Organization = organizationOf(orgID, orgName)
	return g, err
}

func scanOwnGroup(row pgx.CollectableRow) (accesstypes.OwnGroup, error) {
	var o accesstypes.OwnGroup
	var orgID *uuid.UUID
	var orgName *string
	g := &o.Group
	err := row.Scan(&g.ID, &g.Name, &g.MemberCount, &g.OnCallURL, &g.CreatedAt, &g.UpdatedAt, &orgID, &orgName, &o.CanEditContacts)
	g.Contacts = []accesstypes.Contact{}
	g.Organization = organizationOf(orgID, orgName)
	return o, err
}

func organizationOf(id *uuid.UUID, name *string) *accesstypes.NodeRef {
	if id == nil {
		return nil
	}
	return &accesstypes.NodeRef{ID: *id, Name: name}
}

func scanMembership(row pgx.CollectableRow) (accesstypes.Membership, error) {
	var m accesstypes.Membership
	err := row.Scan(&m.GroupExists, &m.UserExists, &m.Member, &m.Editor)
	return m, err
}

func scanIdentity(row pgx.CollectableRow) (accesstypes.Identity, error) {
	return scanIdentityWith(row)
}

func scanIdentityWith(row pgx.Row, extra ...any) (accesstypes.Identity, error) {
	var i accesstypes.Identity
	err := row.Scan(append([]any{&i.ID, &i.UserID, &i.Provider, &i.Subject, &i.Email, &i.CreatedAt, &i.LastLoginAt}, extra...)...)
	return i, err
}

type identityUser struct {
	identity accesstypes.Identity
	user     accesstypes.User
}

func scanIdentityUser(row pgx.CollectableRow) (identityUser, error) {
	var out identityUser
	var u accesstypes.User
	var st string
	i, err := scanIdentityWith(row, &u.ID, &u.Email, &u.DisplayName, &st, &u.IsSuperadmin, &u.IsService, &u.HasPassword, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return out, err
	}
	if u.Status, err = status(st); err != nil {
		return out, err
	}
	return identityUser{identity: i, user: u}, nil
}

func scanMember(row pgx.CollectableRow) (accesstypes.Member, error) {
	var m accesstypes.Member
	var st, source string
	if err := row.Scan(&m.User.ID, &m.User.Email, &m.User.DisplayName, &st, &source, &m.EditorSource); err != nil {
		return m, err
	}
	m.ContactEditor = m.EditorSource != nil
	var err error
	m.User.Status, err = status(st)
	m.Source = accesstypes.MembershipSource(source)
	return m, err
}

func scanAppGrant(r pgx.CollectableRow) (accesstypes.AppGrant, error) {
	var g accesstypes.AppGrant
	var scopes, redirects []string
	var st string
	err := r.Scan(&g.ID, &g.UserID, &g.ClientID, &g.ClientName, &g.ClientMetadataURL, &redirects, &scopes, &g.NodeIDs, &g.CreatedAt,
		&g.LastUsedAt, &g.ExpiresAt, &g.RevokedAt, &st)
	if err != nil {
		return g, err
	}
	if len(redirects) > 0 {
		g.RedirectHost = accesstypes.RedirectHost(redirects[0])
	}
	var ok bool
	if g.Status, ok = accesstypes.ParseTokenStatus(st); !ok {
		return g, apperr.InternalDetail("unknown grant status " + st)
	}
	g.Scopes, err = parseScopes(scopes)
	return g, err
}
