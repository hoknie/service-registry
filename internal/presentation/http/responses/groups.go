package responses

import (
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
)

type Contact struct {
	Kind  string  `json:"kind"`
	Value string  `json:"value"`
	Label *string `json:"label"`
}

func ContactsOf(in []accesstypes.Contact) []Contact {
	out := make([]Contact, 0, len(in))
	for _, c := range in {
		out = append(out, Contact{Kind: c.Kind, Value: c.Value, Label: c.Label})
	}
	return out
}

type Group struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	MemberCount int64     `json:"member_count"`
	Contacts    []Contact `json:"contacts"`
	OnCallURL   *string   `json:"on_call_url"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

func GroupOf(g accesstypes.Group) Group {
	return Group{ID: g.ID, Name: g.Name, MemberCount: g.MemberCount, Contacts: ContactsOf(g.Contacts), OnCallURL: g.OnCallURL,
		CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt}
}

type Member struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	DisplayName   string    `json:"display_name"`
	Status        string    `json:"status"`
	Source        string    `json:"source"`
	ContactEditor bool      `json:"contact_editor"`
	EditorSource  *string   `json:"contact_editor_source"`
}

type GroupDetails struct {
	Group
	Members []Member `json:"members"`
}

func GroupDetailsOf(d accesstypes.GroupDetails) GroupDetails {
	members := make([]Member, 0, len(d.Members))
	for _, m := range d.Members {
		members = append(members, Member{ID: m.User.ID, Email: m.User.Email, DisplayName: m.User.DisplayName,
			Status: string(m.User.Status), Source: string(m.Source), ContactEditor: m.ContactEditor,
			EditorSource: (*string)(m.EditorSource)})
	}
	return GroupDetails{Group: GroupOf(d.Group), Members: members}
}

type OwnGroup struct {
	Group
	CanEditContacts bool `json:"can_edit_contacts"`
}

func OwnGroupOf(g accesstypes.OwnGroup) OwnGroup {
	return OwnGroup{Group: GroupOf(g.Group), CanEditContacts: g.CanEditContacts}
}
