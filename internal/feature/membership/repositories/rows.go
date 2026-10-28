package repositories

import (
	"github.com/google/uuid"

	membershiptypes "svc-registry/internal/feature/membership/types"
)

type memberRow struct {
	ID          uuid.UUID  `db:"id"`
	Name        string     `db:"name"`
	Email       string     `db:"email"`
	Service     bool       `db:"service"`
	Source      string     `db:"source"`
	AddedByID   *uuid.UUID `db:"added_by_id"`
	AddedByName *string    `db:"added_by_name"`
	AddedAt     string     `db:"added_at"`
	Cursor      string     `db:"cursor"`
}

func (r memberRow) member() membershiptypes.Member {
	m := membershiptypes.Member{User: membershiptypes.Person{ID: r.ID, Name: r.Name, Email: r.Email, Service: r.Service},
		Source: r.Source, AddedAt: r.AddedAt, Cursor: r.Cursor}
	if r.AddedByID != nil {
		m.AddedBy = &membershiptypes.Person{ID: *r.AddedByID, Name: deref(r.AddedByName)}
	}
	return m
}

type personRow struct {
	ID      uuid.UUID `db:"id"`
	Name    string    `db:"name"`
	Email   string    `db:"email"`
	Service bool      `db:"service"`
}

func (r personRow) person() membershiptypes.Person {
	return membershiptypes.Person{ID: r.ID, Name: r.Name, Email: r.Email, Service: r.Service}
}

type invitationRow struct {
	ID             uuid.UUID  `db:"id"`
	OrganizationID uuid.UUID  `db:"organization_id"`
	Organization   string     `db:"organization"`
	Email          string     `db:"email"`
	Role           string     `db:"role"`
	Locale         string     `db:"locale"`
	InvitedByID    *uuid.UUID `db:"invited_by_id"`
	InvitedByName  *string    `db:"invited_by_name"`
	CreatedAt      string     `db:"created_at"`
	ExpiresAt      string     `db:"expires_at"`
	AcceptedAt     *string    `db:"accepted_at"`
	RevokedAt      *string    `db:"revoked_at"`
	Expired        bool       `db:"expired"`
}

func (r invitationRow) invitation() membershiptypes.Invitation {
	i := membershiptypes.Invitation{ID: r.ID, OrganizationID: r.OrganizationID, Organization: r.Organization, Email: r.Email,
		Role: membershiptypes.Role(r.Role), Locale: r.Locale, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt,
		AcceptedAt: r.AcceptedAt, RevokedAt: r.RevokedAt, Expired: r.Expired}
	if r.InvitedByID != nil {
		i.InvitedBy = &membershiptypes.Person{ID: *r.InvitedByID, Name: deref(r.InvitedByName)}
	}
	return i
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
