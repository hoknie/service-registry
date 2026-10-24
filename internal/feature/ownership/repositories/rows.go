package repositories

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	ownershiptypes "svc-registry/internal/feature/ownership/types"
)

func kindOf(isGroup bool) catalogtypes.SubjectKind {
	if isGroup {
		return catalogtypes.SubjectGroup
	}
	return catalogtypes.SubjectUser
}

func scanOwner(row pgx.CollectableRow) (ownershiptypes.Owner, error) {
	var o ownershiptypes.Owner
	var isGroup bool
	err := row.Scan(&o.NodeID, &o.Inherited, &isGroup, &o.SubjectID, &o.Name, &o.Email, &o.Status, &o.OnCallURL)
	o.SubjectKind = kindOf(isGroup)
	o.Contacts = []accesstypes.Contact{}
	return o, err
}

type badgeRow struct {
	nodeID uuid.UUID
	badge  ownershiptypes.Badge
}

func scanBadge(row pgx.CollectableRow) (badgeRow, error) {
	var b badgeRow
	var isGroup bool
	err := row.Scan(&b.nodeID, &isGroup, &b.badge.SubjectID, &b.badge.Name)
	b.badge.SubjectKind = kindOf(isGroup)
	return b, err
}

func scanCandidate(row pgx.CollectableRow) (ownershiptypes.Candidate, error) {
	var c ownershiptypes.Candidate
	var isGroup bool
	err := row.Scan(&isGroup, &c.SubjectID, &c.Name, &c.Email, &c.Members, &c.OrganizationID, &c.OrganizationName)
	c.SubjectKind = kindOf(isGroup)
	return c, err
}

func scanSuggestion(row pgx.CollectableRow) (ownershiptypes.Suggestion, error) {
	var s ownershiptypes.Suggestion
	var isGroup bool
	err := row.Scan(&isGroup, &s.SubjectID, &s.Name, &s.Count)
	s.SubjectKind = kindOf(isGroup)
	return s, err
}

type contactRow struct {
	groupID uuid.UUID
	contact accesstypes.Contact
}

func scanContact(row pgx.CollectableRow) (contactRow, error) {
	var c contactRow
	err := row.Scan(&c.groupID, &c.contact.Kind, &c.contact.Value, &c.contact.Label)
	return c, err
}

type resolvedRow struct {
	groupID *uuid.UUID
	userID  *uuid.UUID
}

func scanResolved(row pgx.CollectableRow) (resolvedRow, error) {
	var r resolvedRow
	err := row.Scan(&r.groupID, &r.userID)
	return r, err
}
