package types

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxFolderDepth = 8

var folderSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

type Ref struct {
	ID   uuid.UUID
	Slug string
	Name string
}

type Scope struct {
	OrganizationID *uuid.UUID
}

func (s Scope) Global() bool { return s.OrganizationID == nil }

type Folder struct {
	ID             uuid.UUID
	ParentID       *uuid.UUID
	OrganizationID *uuid.UUID
	Organization   *Ref
	Slug           string
	Name           string
	Description    string
	Path           string
	Ancestors      []Ref
	Depth          int
	Servers        int64
	Folders        int64
	CanWrite       bool
	CreatedAt      string
	UpdatedAt      string
}

type NewFolder struct {
	ID             uuid.UUID
	ParentID       *uuid.UUID
	OrganizationID *uuid.UUID
	Slug           string
	Name           string
	Description    string
}

func CleanFolder(slug, name, description string) (string, string, string, error) {
	name, description = strings.TrimSpace(name), strings.TrimSpace(description)
	if !folderSlug.MatchString(slug) || name == "" || utf8.RuneCountInString(name) > 100 || utf8.RuneCountInString(description) > 2000 {
		return "", "", "", InvalidFolder
	}
	return slug, name, description, nil
}

type Visible struct {
	Organizations []uuid.UUID
	Global        bool
}

func (v Visible) Has(s Scope) bool {
	if s.OrganizationID == nil {
		return v.Global
	}
	for _, id := range v.Organizations {
		if id == *s.OrganizationID {
			return true
		}
	}
	return false
}
