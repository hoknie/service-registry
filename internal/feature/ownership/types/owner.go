package types

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
)

const (
	MaxOwners      = 10
	MaxCandidates  = 20
	MaxSuggestions = 20
	QueryMaxLen    = 100
)

type Owner struct {
	SubjectKind catalogtypes.SubjectKind
	SubjectID   uuid.UUID
	Name        string
	Email       *string
	Status      *string
	OnCallURL   *string
	Contacts    []accesstypes.Contact
	NodeID      uuid.UUID
	Inherited   bool
}

type OwnerSet struct {
	Items []Owner
	Own   bool
}

type PutOwner struct {
	SubjectKind string
	Subject     string
}

type NewOwner struct {
	SubjectKind catalogtypes.SubjectKind
	Subject     string
}

type Badge struct {
	SubjectKind catalogtypes.SubjectKind
	SubjectID   uuid.UUID
	Name        string
}

type Candidate struct {
	SubjectKind      catalogtypes.SubjectKind
	SubjectID        uuid.UUID
	Name             string
	Email            *string
	Members          *int64
	OrganizationID   *uuid.UUID
	OrganizationName *string
}

type Suggestion struct {
	SubjectKind catalogtypes.SubjectKind
	SubjectID   uuid.UUID
	Name        string
	Count       int64
}

func ValidateOwners(in []PutOwner, none bool) ([]NewOwner, error) {
	if len(in) > MaxOwners || none {
		return nil, InvalidOwners
	}
	seen := map[string]bool{}
	out := make([]NewOwner, 0, len(in))
	for _, o := range in {
		kind, ok := catalogtypes.ParseSubjectKind(o.SubjectKind)
		if !ok {
			return nil, catalogtypes.InvalidSubjectKind
		}
		subject := strings.TrimFunc(o.Subject, unicode.IsSpace)
		key := string(kind) + ":" + strings.ToLower(subject)
		if seen[key] {
			return nil, InvalidOwners
		}
		seen[key] = true
		out = append(out, NewOwner{SubjectKind: kind, Subject: subject})
	}
	return out, nil
}

func ValidateCandidateQuery(q string) (string, error) {
	q = strings.TrimFunc(q, unicode.IsSpace)
	if utf8.RuneCountInString(q) > QueryMaxLen || hasControl(q) {
		return "", catalogtypes.InvalidFilter
	}
	return q, nil
}

func ValidateSuggestQuery(q string) (string, error) {
	q = strings.TrimFunc(q, unicode.IsSpace)
	if utf8.RuneCountInString(q) > QueryMaxLen || hasControl(q) {
		return "", catalogtypes.InvalidFilter
	}
	return q, nil
}

func hasControl(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) >= 0
}

func SetOf(items []Owner) OwnerSet {
	out := OwnerSet{Items: items}
	if out.Items == nil {
		out.Items = []Owner{}
	}
	for _, o := range out.Items {
		out.Own = out.Own || !o.Inherited
	}
	return out
}
