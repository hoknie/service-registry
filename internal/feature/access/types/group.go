package types

import "github.com/google/uuid"

type Group struct {
	ID          uuid.UUID
	Name        string
	MemberCount int64
	OnCallURL   *string
	Contacts    []Contact
	CreatedAt   string
	UpdatedAt   string
}

type MembershipSource string

const SourceManual MembershipSource = "manual"

func OAuthSource(provider string) MembershipSource { return MembershipSource("oauth:" + provider) }

type Member struct {
	User          UserSummary
	Source        MembershipSource
	ContactEditor bool
	EditorSource  *MembershipSource
}

type OwnGroup struct {
	Group           Group
	CanEditContacts bool
}

type GroupDetails struct {
	Group   Group
	Members []Member
}

type Membership struct {
	GroupExists bool
	UserExists  bool
	Member      bool
	Editor      bool
}
