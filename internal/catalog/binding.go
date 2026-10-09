package catalog

import "github.com/google/uuid"

type SubjectKind string

const (
	SubjectUser  SubjectKind = "user"
	SubjectGroup SubjectKind = "group"
)

func ParseSubjectKind(s string) (SubjectKind, bool) {
	switch SubjectKind(s) {
	case SubjectUser, SubjectGroup:
		return SubjectKind(s), true
	}
	return "", false
}

type Binding struct {
	ID          uuid.UUID
	NodeID      uuid.UUID
	NodeName    string
	SubjectKind SubjectKind
	SubjectID   uuid.UUID
	SubjectName string
	Role        Role
	CreatedAt   string
}
