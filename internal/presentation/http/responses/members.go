package responses

import (
	"github.com/google/uuid"

	membershipservices "svc-registry/internal/feature/membership/services"
	membershiptypes "svc-registry/internal/feature/membership/types"
)

type MemberPerson struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Email   string    `json:"email"`
	Service bool      `json:"service"`
}

type MemberRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type OrgMember struct {
	User    MemberPerson `json:"user"`
	Role    *string      `json:"role"`
	Source  string       `json:"source"`
	AddedBy *MemberRef   `json:"added_by"`
	AddedAt string       `json:"added_at"`
}

type OrgMembers struct {
	Items      []OrgMember `json:"items"`
	NextCursor *string     `json:"next_cursor"`
	CanManage  bool        `json:"can_manage"`
}

func personOf(p membershiptypes.Person) MemberPerson {
	return MemberPerson{ID: p.ID, Name: p.Name, Email: p.Email, Service: p.Service}
}

func refOf(p *membershiptypes.Person) *MemberRef {
	if p == nil {
		return nil
	}
	return &MemberRef{ID: p.ID, Name: p.Name}
}

func OrgMemberOf(m membershiptypes.Member) OrgMember {
	out := OrgMember{User: personOf(m.User), Source: m.Source, AddedBy: refOf(m.AddedBy), AddedAt: m.AddedAt}
	if m.Role != nil {
		role := string(*m.Role)
		out.Role = &role
	}
	return out
}

func OrgMembersOf(p membershiptypes.MemberPage) OrgMembers {
	out := OrgMembers{Items: make([]OrgMember, len(p.Items)), NextCursor: p.NextCursor, CanManage: p.CanManage}
	for i, m := range p.Items {
		out.Items[i] = OrgMemberOf(m)
	}
	return out
}

type MemberCandidates struct {
	Items []MemberPerson `json:"items"`
}

func MemberCandidatesOf(people []membershiptypes.Person) MemberCandidates {
	out := MemberCandidates{Items: make([]MemberPerson, len(people))}
	for i, p := range people {
		out.Items[i] = personOf(p)
	}
	return out
}

type GroupAccess struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Role  string    `json:"role"`
	Users int64     `json:"users"`
}

type GroupsAccess struct {
	Items []GroupAccess `json:"items"`
}

func GroupsAccessOf(groups []membershiptypes.GroupAccess) GroupsAccess {
	out := GroupsAccess{Items: make([]GroupAccess, len(groups))}
	for i, g := range groups {
		out.Items[i] = GroupAccess{ID: g.ID, Name: g.Name, Role: string(g.Role), Users: g.Users}
	}
	return out
}

type Invitation struct {
	ID        uuid.UUID  `json:"id"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	Locale    string     `json:"locale"`
	InvitedBy *MemberRef `json:"invited_by"`
	CreatedAt string     `json:"created_at"`
	ExpiresAt string     `json:"expires_at"`
	State     string     `json:"state"`
}

func InvitationOf(i membershiptypes.Invitation) Invitation {
	return Invitation{ID: i.ID, Email: i.Email, Role: string(i.Role), Locale: i.Locale, InvitedBy: refOf(i.InvitedBy),
		CreatedAt: i.CreatedAt, ExpiresAt: i.ExpiresAt, State: string(i.State())}
}

type Invitations struct {
	Items []Invitation `json:"items"`
}

func InvitationsOf(list []membershiptypes.Invitation) Invitations {
	out := Invitations{Items: make([]Invitation, len(list))}
	for i, inv := range list {
		out.Items[i] = InvitationOf(inv)
	}
	return out
}

type IssuedInvitation struct {
	Invitation Invitation `json:"invitation"`
	Link       string     `json:"link"`
	Mail       string     `json:"mail"`
}

func IssuedOf(i membershipservices.Issued) IssuedInvitation {
	return IssuedInvitation{Invitation: InvitationOf(i.Invitation), Link: i.Link, Mail: string(i.Mail)}
}

type InvitationProvider struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type InvitationName struct {
	Name string `json:"name"`
}

type InvitationView struct {
	Organization  MemberRef            `json:"organization"`
	Email         string               `json:"email"`
	Role          string               `json:"role"`
	InvitedBy     *InvitationName      `json:"invited_by"`
	ExpiresAt     string               `json:"expires_at"`
	State         string               `json:"state"`
	Account       string               `json:"account"`
	PasswordLogin bool                 `json:"password_login"`
	Providers     []InvitationProvider `json:"providers"`
}

func InvitationViewOf(v membershipservices.InvitationView) InvitationView {
	i := v.Invitation
	out := InvitationView{Organization: MemberRef{ID: i.OrganizationID, Name: i.Organization}, Email: i.Email, Role: string(i.Role),
		ExpiresAt: i.ExpiresAt, State: string(i.State()), Account: "new", PasswordLogin: v.PasswordLogin,
		Providers: make([]InvitationProvider, len(v.Providers))}
	if v.AccountExists {
		out.Account = "exists"
	}
	if i.InvitedBy != nil {
		out.InvitedBy = &InvitationName{Name: i.InvitedBy.Name}
	}
	for k, p := range v.Providers {
		out.Providers[k] = InvitationProvider{Key: p.Key, Name: p.DisplayName}
	}
	return out
}

type InvitationAccepted struct {
	Organization MemberRef `json:"organization"`
}

func InvitationAcceptedOf(i membershiptypes.Invitation) InvitationAccepted {
	return InvitationAccepted{Organization: MemberRef{ID: i.OrganizationID, Name: i.Organization}}
}
