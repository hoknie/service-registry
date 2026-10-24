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
	ID           uuid.UUID          `json:"id"`
	Name         string             `json:"name"`
	Organization *GroupOrganization `json:"organization"`
	MemberCount  int64              `json:"member_count"`
	Contacts     []Contact          `json:"contacts"`
	OnCallURL    *string            `json:"on_call_url"`
	CreatedAt    string             `json:"created_at"`
	UpdatedAt    string             `json:"updated_at"`
}

type GroupOrganization struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func GroupOrganizationOf(ref *accesstypes.NodeRef) *GroupOrganization {
	if ref == nil {
		return nil
	}
	out := &GroupOrganization{ID: ref.ID}
	if ref.Name != nil {
		out.Name = *ref.Name
	}
	return out
}

func GroupOf(g accesstypes.Group) Group {
	return Group{ID: g.ID, Name: g.Name, Organization: GroupOrganizationOf(g.Organization), MemberCount: g.MemberCount,
		Contacts: ContactsOf(g.Contacts), OnCallURL: g.OnCallURL, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt}
}

type OrganizationGroups struct {
	Page[Group]
	Organization *GroupOrganization `json:"organization"`
	CanManage    bool               `json:"can_manage"`
}

func OrganizationGroupsOf(l accesstypes.GroupList) OrganizationGroups {
	return OrganizationGroups{Page: PageOf(l.Page, GroupOf), Organization: GroupOrganizationOf(l.Organization), CanManage: l.CanManage}
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
	Members          []Member `json:"members"`
	Permissions      Rights   `json:"permissions"`
	MembersLock      *string  `json:"members_lock"`
	CanManageMembers bool     `json:"can_manage_members"`
	MembersHidden    bool     `json:"members_hidden"`
}

type Rights map[string][]string

func RightsOf(r accesstypes.Rights) Rights {
	out := Rights{}
	for section, actions := range r.Union(nil) {
		names := make([]string, len(actions))
		for i, a := range actions {
			names[i] = string(a)
		}
		out[string(section)] = names
	}
	return out
}

type GroupPermissions struct {
	Permissions Rights `json:"permissions"`
}

func GroupDetailsOf(d accesstypes.GroupDetails) GroupDetails {
	members := make([]Member, 0, len(d.Members))
	for _, m := range d.Members {
		members = append(members, Member{ID: m.User.ID, Email: m.User.Email, DisplayName: m.User.DisplayName,
			Status: string(m.User.Status), Source: string(m.Source), ContactEditor: m.ContactEditor,
			EditorSource: (*string)(m.EditorSource)})
	}
	var lock *string
	if d.MembersLock != accesstypes.LockNone {
		lock = new(string(d.MembersLock))
	}
	return GroupDetails{Group: GroupOf(d.Group), Members: members, Permissions: RightsOf(d.Permissions), MembersLock: lock,
		CanManageMembers: d.CanManageMembers, MembersHidden: d.MembersHidden}
}

type ManagedGroup struct {
	Group
	Nodes []TokenNode `json:"nodes"`
}

func ManagedGroupOf(m accesstypes.ManagedGroup) ManagedGroup {
	nodes := make([]TokenNode, len(m.Nodes))
	for i, n := range m.Nodes {
		nodes[i] = TokenNode{ID: n.ID, Kind: n.Kind, Name: n.Name, Path: n.Path}
	}
	return ManagedGroup{Group: GroupOf(m.Group), Nodes: nodes}
}

type ManagedGroupDetails struct {
	Group
	Members          []Member `json:"members"`
	MembersLock      *string  `json:"members_lock"`
	CanManageMembers bool     `json:"can_manage_members"`
}

func ManagedGroupDetailsOf(d accesstypes.GroupDetails) ManagedGroupDetails {
	full := GroupDetailsOf(d)
	return ManagedGroupDetails{Group: full.Group, Members: full.Members, MembersLock: full.MembersLock, CanManageMembers: full.CanManageMembers}
}

type MemberCandidate struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

func MemberCandidateOf(c accesstypes.MemberCandidate) MemberCandidate {
	return MemberCandidate{ID: c.ID, DisplayName: c.DisplayName, Email: c.Email}
}

type OwnGroup struct {
	Group
	CanEditContacts bool `json:"can_edit_contacts"`
}

func OwnGroupOf(g accesstypes.OwnGroup) OwnGroup {
	return OwnGroup{Group: GroupOf(g.Group), CanEditContacts: g.CanEditContacts}
}
