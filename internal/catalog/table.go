package catalog

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
}

func (f TableFilter) Active() bool {
	return f.Q != "" || f.Kind != "" || len(f.Labels) > 0 || f.Activity != ""
}

type Search struct {
	UserID       uuid.UUID
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
	return f, nil
}
