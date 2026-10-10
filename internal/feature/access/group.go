package access

import "github.com/google/uuid"

type Group struct {
	ID          uuid.UUID
	Name        string
	MemberCount int64
	CreatedAt   string
	UpdatedAt   string
}

type MembershipSource string

const SourceManual MembershipSource = "manual"

func OAuthSource(provider string) MembershipSource { return MembershipSource("oauth:" + provider) }

type Member struct {
	User   UserSummary
	Source MembershipSource
}

type GroupDetails struct {
	Group   Group
	Members []Member
}
