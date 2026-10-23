package types

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Source string

const (
	SourceEvent      Source = "event"
	SourceCluster    Source = "cluster"
	SourceRepository Source = "repository"
	SourceManual     Source = "manual"
)

const (
	TargetProject    = "project"
	TargetHidden     = "hidden"
	TargetExternal   = "external"
	TargetUnresolved = "unresolved"
	MaxNoteLen       = 500
	MaxDepth         = 3
	MaxGraphNodes    = 500
	MaxClusterValue  = 4096
)

type Edge struct {
	ID              uuid.UUID
	ProjectID       uuid.UUID
	Source          Source
	Origin          string
	TargetProjectID *uuid.UUID
	TargetExternal  *string
	TargetRef       string
	Note            *string
	SeenAt          string
}

func (e Edge) TargetKey() string {
	switch {
	case e.TargetProjectID != nil:
		return e.TargetProjectID.String()
	case e.TargetExternal != nil:
		return "external:" + *e.TargetExternal
	}
	return "ref:" + e.TargetRef
}

type NewTarget struct {
	ProjectID *uuid.UUID
	External  *string
	Ref       string
}

type ClusterEdges struct {
	ProjectID uuid.UUID
	Origin    string
	Targets   []NewTarget
}

type SourceRef struct {
	Source Source
	Origin string
	SeenAt string
	ID     *uuid.UUID
	Note   *string
}

type ProjectRef struct {
	ID   uuid.UUID
	Path string
	Name string
}

type Target struct {
	Type     string
	Project  *ProjectRef
	External string
	Ref      string
}

type Item struct {
	Target  Target
	Sources []SourceRef
}

type Dependent struct {
	Project ProjectRef
	Sources []SourceRef
}

type List struct {
	DependsOn        []Item
	Dependents       []Dependent
	HiddenDependents int
}

type GraphNode struct {
	ID      string
	Type    string
	Path    string
	Name    string
	InScope bool
}

type GraphEdge struct {
	From    string
	To      string
	Sources []Source
}

type Graph struct {
	Nodes       []GraphNode
	Edges       []GraphEdge
	HiddenEdges int
	Truncated   bool
}

func SourceOf(e Edge) SourceRef {
	s := SourceRef{Source: e.Source, Origin: e.Origin, SeenAt: e.SeenAt, Note: e.Note}
	if e.Source == SourceManual {
		s.ID = new(e.ID)
	}
	return s
}

func targetRank(t Target) (int, string) {
	switch t.Type {
	case TargetProject:
		return 0, t.Project.Path
	case TargetExternal:
		return 1, t.External
	case TargetUnresolved:
		return 2, t.Ref
	}
	return 3, ""
}

func SortItems(items []Item) {
	slices.SortStableFunc(items, func(a, b Item) int {
		ra, ka := targetRank(a.Target)
		rb, kb := targetRank(b.Target)
		if ra != rb {
			return ra - rb
		}
		return strings.Compare(ka, kb)
	})
}

func ValidNote(raw *string) (*string, bool) {
	if raw == nil {
		return nil, true
	}
	s := strings.TrimSpace(*raw)
	if s == "" {
		return nil, true
	}
	if utf8.RuneCountInString(s) > MaxNoteLen || strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) && r != '\n' }) {
		return nil, false
	}
	return &s, true
}

func ClusterValue(s string) []string {
	if r := []rune(s); len(r) > MaxClusterValue {
		s = string(r[:MaxClusterValue])
	}
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	if len(out) > MaxRefs {
		out = out[:MaxRefs]
	}
	return out
}

func ClusterOrigin(cluster uuid.UUID, namespace, kind, name string) string {
	return cluster.String() + "/" + namespace + "/" + kind + "/" + name
}
