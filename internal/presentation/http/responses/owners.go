package responses

import (
	"github.com/google/uuid"

	ownershiptypes "svc-registry/internal/feature/ownership/types"
)

type Owner struct {
	SubjectKind string    `json:"subject_kind"`
	SubjectID   uuid.UUID `json:"subject_id"`
	Name        string    `json:"name"`
	Email       *string   `json:"email"`
	Status      *string   `json:"status"`
	Contacts    []Contact `json:"contacts"`
	OnCallURL   *string   `json:"on_call_url"`
	NodeID      uuid.UUID `json:"node_id"`
	Inherited   bool      `json:"inherited"`
}

type OwnerSet struct {
	Items []Owner    `json:"items"`
	From  *uuid.UUID `json:"from"`
	Own   bool       `json:"own"`
	None  bool       `json:"none"`
}

func OwnerSetOf(s ownershiptypes.OwnerSet) OwnerSet {
	items := make([]Owner, 0, len(s.Items))
	for _, o := range s.Items {
		items = append(items, Owner{SubjectKind: string(o.SubjectKind), SubjectID: o.SubjectID, Name: o.Name, Email: o.Email,
			Status: o.Status, Contacts: ContactsOf(o.Contacts), OnCallURL: o.OnCallURL, NodeID: o.NodeID, Inherited: o.Inherited})
	}
	return OwnerSet{Items: items, From: s.From, Own: s.Own, None: s.None}
}

type OwnerBadge struct {
	SubjectKind string    `json:"subject_kind"`
	SubjectID   uuid.UUID `json:"subject_id"`
	Name        string    `json:"name"`
}

func OwnerBadgesOf(in []ownershiptypes.Badge) []OwnerBadge {
	out := make([]OwnerBadge, 0, len(in))
	for _, b := range in {
		out = append(out, OwnerBadge{SubjectKind: string(b.SubjectKind), SubjectID: b.SubjectID, Name: b.Name})
	}
	return out
}

type OwnerCandidate struct {
	SubjectKind  string             `json:"subject_kind"`
	SubjectID    uuid.UUID          `json:"subject_id"`
	Name         string             `json:"name"`
	Email        *string            `json:"email"`
	Members      *int64             `json:"members"`
	Organization *GroupOrganization `json:"organization"`
}

func OwnerCandidateOf(c ownershiptypes.Candidate) OwnerCandidate {
	var org *GroupOrganization
	if c.OrganizationID != nil {
		org = &GroupOrganization{ID: *c.OrganizationID}
		if c.OrganizationName != nil {
			org.Name = *c.OrganizationName
		}
	}
	return OwnerCandidate{SubjectKind: string(c.SubjectKind), SubjectID: c.SubjectID, Name: c.Name, Email: c.Email, Members: c.Members,
		Organization: org}
}

type OwnerSuggestion struct {
	SubjectKind string    `json:"subject_kind"`
	SubjectID   uuid.UUID `json:"subject_id"`
	Name        string    `json:"name"`
	Count       int64     `json:"count"`
}

func OwnerSuggestionOf(s ownershiptypes.Suggestion) OwnerSuggestion {
	return OwnerSuggestion{SubjectKind: string(s.SubjectKind), SubjectID: s.SubjectID, Name: s.Name, Count: s.Count}
}
