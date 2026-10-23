package types

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaxRefLen      = 1024
	MaxRefs        = 50
	externalPrefix = "external:"
)

var (
	ErrInvalidRef = errors.New("invalid dependency reference")
	externalName  = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]{0,61}[a-z0-9])?$`)
)

type Ref struct {
	ID       *uuid.UUID
	Path     []string
	External string
}

func ParseRef(raw string) (Ref, error) {
	s := strings.TrimSpace(raw)
	if s == "" || utf8.RuneCountInString(s) > MaxRefLen {
		return Ref{}, ErrInvalidRef
	}
	if len(s) >= len(externalPrefix) && strings.EqualFold(s[:len(externalPrefix)], externalPrefix) {
		name := strings.ToLower(strings.TrimSpace(s[len(externalPrefix):]))
		if !externalName.MatchString(name) {
			return Ref{}, ErrInvalidRef
		}
		return Ref{External: name}, nil
	}
	if id, err := uuid.Parse(s); err == nil {
		return Ref{ID: &id}, nil
	}
	path := strings.Split(strings.ToLower(strings.Trim(s, "/")), "/")
	for _, seg := range path {
		if seg == "" || strings.ContainsFunc(seg, func(r rune) bool { return r <= ' ' || r == 0x7f }) {
			return Ref{}, ErrInvalidRef
		}
	}
	return Ref{Path: path}, nil
}

func (r Ref) Text() string {
	switch {
	case r.External != "":
		return externalPrefix + r.External
	case r.ID != nil:
		return r.ID.String()
	}
	return strings.Join(r.Path, "/")
}

func ParseRefs(raw []string) (refs []Ref, invalid int) {
	seen := map[string]bool{}
	for _, s := range raw {
		r, err := ParseRef(s)
		if err != nil {
			invalid++
			continue
		}
		if t := r.Text(); !seen[t] {
			seen[t] = true
			refs = append(refs, r)
		}
	}
	return refs, invalid
}
