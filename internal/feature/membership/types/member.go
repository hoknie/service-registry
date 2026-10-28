package types

import (
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	Viewer Role = "viewer"
	Editor Role = "editor"
	Admin  Role = "admin"
)

var roleRank = map[Role]int{Viewer: 1, Editor: 2, Admin: 3}

func ParseRole(raw string) (Role, bool) {
	r := Role(strings.TrimSpace(raw))
	_, ok := roleRank[r]
	return r, ok
}

func Higher(current *Role, offered Role) Role {
	if current != nil && roleRank[*current] >= roleRank[offered] {
		return *current
	}
	return offered
}

type Source string

const (
	SourceManual     Source = "manual"
	SourceInvitation Source = "invitation"
)

var Locales = []string{"en", "es", "ru", "zh"}

func ValidLocale(locale string) bool { return slices.Contains(Locales, locale) }

func NormalizeEmail(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || len(email) > 254 || addr.Name != "" {
		return "", false
	}
	return email, true
}

type Person struct {
	ID      uuid.UUID
	Name    string
	Email   string
	Service bool
}

type Member struct {
	User    Person
	Role    *Role
	Source  string
	AddedBy *Person
	AddedAt string
	Cursor  string
}

type MemberPage struct {
	Items      []Member
	NextCursor *string
	CanManage  bool
}

type GroupAccess struct {
	ID    uuid.UUID
	Name  string
	Role  Role
	Users int64
}

type State string

const (
	Pending  State = "pending"
	Expired  State = "expired"
	Accepted State = "accepted"
	Revoked  State = "revoked"
)

type Invitation struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Organization   string
	Email          string
	Role           Role
	Locale         string
	InvitedBy      *Person
	CreatedAt      string
	ExpiresAt      string
	AcceptedAt     *string
	RevokedAt      *string
	Expired        bool
}

func (i Invitation) State() State {
	switch {
	case i.AcceptedAt != nil:
		return Accepted
	case i.RevokedAt != nil:
		return Revoked
	case i.Expired:
		return Expired
	}
	return Pending
}

type NewInvitation struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Email          string
	Role           Role
	Locale         string
	TokenHash      [32]byte
	InvitedBy      uuid.UUID
	ExpiresAt      time.Time
}

type MailState string

const (
	MailSent     MailState = "sent"
	MailDisabled MailState = "disabled"
	MailFailed   MailState = "failed"
)

const MaxPendingInvitations = 100
