package types

import (
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	SetAll      = "all"
	SetSelected = "selected"
)

type GroupRef struct {
	ID   uuid.UUID
	Name string
}

type UserRef struct {
	ID    uuid.UUID
	Email string
	Name  string
}

type AccessList struct {
	Everyone  bool
	Groups    []GroupRef
	Users     []UserRef
	CanManage bool
}

type ServerRef struct {
	ID             uuid.UUID
	Slug           string
	Title          string
	OrganizationID *uuid.UUID
	Status         Status
}

type OrgSet struct {
	Mode      string
	Servers   []ServerRef
	CanManage bool
}

func CleanIDs(ids []uuid.UUID, max int) ([]uuid.UUID, bool) {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			return nil, false
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, len(out) <= max
}

const (
	CallOK        = "ok"
	CallToolError = "tool_error"
	CallFailed    = "failed"
)

type Caller struct {
	UserID  uuid.UUID
	TokenID *uuid.UUID
	GrantID *uuid.UUID
}

type CallRecord struct {
	ID         uuid.UUID
	ServerID   uuid.UUID
	Tool       string
	Caller     Caller
	DurationMs int
	Outcome    string
	Code       *string
	Request    *string
	Response   *string
	Truncated  bool
}

type CallEntry struct {
	ID         uuid.UUID
	At         string
	Cursor     string
	Tool       string
	UserID     *uuid.UUID
	UserEmail  *string
	UserName   *string
	ViaKind    *string
	ViaID      *uuid.UUID
	ViaName    *string
	DurationMs int
	Outcome    string
	Code       *string
	Request    *string
	Response   *string
	Truncated  bool
}

type Viewer struct {
	UserID        uuid.UUID
	Read          []uuid.UUID
	Write         Visible
	Organizations []uuid.UUID
	Limited       bool
	Servers       []uuid.UUID
}

type Available struct {
	Server Server
	Tools  []Tool
}

const MaxPayload = 16384

func Clip(text string) (string, bool) {
	if len(text) <= MaxPayload {
		return text, false
	}
	cut := MaxPayload
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut], true
}
