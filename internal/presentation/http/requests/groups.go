package requests

import (
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
)

type GroupName struct {
	Name *string `json:"name" validate:"required"`
}

func (r GroupName) Group(org *uuid.UUID) accesstypes.GroupName {
	return accesstypes.GroupName{Name: *r.Name, Organization: org}
}

type Contact struct {
	Kind  *string `json:"kind" validate:"required"`
	Value *string `json:"value" validate:"required"`
	Label *string `json:"label"`
}

type GroupContacts struct {
	Contacts  *[]Contact `json:"contacts" validate:"required,dive"`
	OnCallURL *string    `json:"on_call_url"`
}

func (r GroupContacts) GroupContacts() accesstypes.GroupContacts {
	out := accesstypes.GroupContacts{OnCallURL: r.OnCallURL, Contacts: make([]accesstypes.Contact, 0, len(*r.Contacts))}
	for _, c := range *r.Contacts {
		out.Contacts = append(out.Contacts, accesstypes.Contact{Kind: *c.Kind, Value: *c.Value, Label: c.Label})
	}
	return out
}

type GroupPermissions struct {
	Permissions *map[string][]string `json:"permissions" validate:"required"`
}
