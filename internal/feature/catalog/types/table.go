package types

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	TableMaxMatches = 500
	TableMaxLabels  = 5
)

type TableQuery struct {
	Q        *string
	Kind     *string
	Labels   []string
	Activity *string
	Owner    *string
}

type LabelFilter struct {
	Key   string
	Value *string
}

type TableFilter struct {
	Q        string
	Kind     NodeKind
	Labels   []LabelFilter
	Activity ProcessState
	Owner    *OwnerFilter
}

type OwnerFilter struct {
	Me   bool
	Kind SubjectKind
	ID   uuid.UUID
}

func (f TableFilter) Active() bool {
	return f.Q != "" || f.Kind != "" || len(f.Labels) > 0 || f.Activity != "" || f.Owner != nil
}

type Search struct {
	Viewer       Viewer
	Root         *uuid.UUID
	RootReadable bool
	Filter       TableFilter
	Only         []uuid.UUID
	Limit        int
}

type TableNode struct {
	WalkNode
	Children int64
	Match    bool
}

func ValidateTable(q TableQuery) (TableFilter, error) {
	var f TableFilter
	if q.Q != nil && *q.Q != "" {
		if utf8.RuneCountInString(*q.Q) > 100 || hasControl(*q.Q) {
			return TableFilter{}, InvalidFilter
		}
		f.Q = *q.Q
	}
	if q.Kind != nil && *q.Kind != "" {
		k, ok := ParseKind(*q.Kind)
		if !ok {
			return TableFilter{}, InvalidFilter
		}
		f.Kind = k
	}
	if len(q.Labels) > TableMaxLabels {
		return TableFilter{}, InvalidFilter
	}
	for _, raw := range q.Labels {
		key, value, has := strings.Cut(raw, "=")
		if !labelKeyOK(key) || (has && (utf8.RuneCountInString(value) > 63 || hasControl(value))) {
			return TableFilter{}, InvalidFilter
		}
		l := LabelFilter{Key: key}
		if has {
			l.Value = &value
		}
		f.Labels = append(f.Labels, l)
	}
	if q.Activity != nil && *q.Activity != "" {
		s, ok := ParseProcessState(*q.Activity)
		if !ok {
			return TableFilter{}, InvalidFilter
		}
		f.Activity = s
	}
	if q.Owner != nil && *q.Owner != "" {
		o, ok := parseOwnerFilter(*q.Owner)
		if !ok {
			return TableFilter{}, InvalidFilter
		}
		f.Owner = &o
	}
	return f, nil
}

func parseOwnerFilter(raw string) (OwnerFilter, bool) {
	if raw == "me" {
		return OwnerFilter{Me: true}, true
	}
	kind, id, ok := strings.Cut(raw, ":")
	k, valid := ParseSubjectKind(kind)
	if !ok || !valid {
		return OwnerFilter{}, false
	}
	u, err := uuid.Parse(id)
	if err != nil {
		return OwnerFilter{}, false
	}
	return OwnerFilter{Kind: k, ID: u}, true
}

func Intersect(a, b []uuid.UUID) []uuid.UUID {
	in := make(map[uuid.UUID]bool, len(b))
	for _, id := range b {
		in[id] = true
	}
	out := []uuid.UUID{}
	for _, id := range a {
		if in[id] {
			out = append(out, id)
		}
	}
	return out
}
