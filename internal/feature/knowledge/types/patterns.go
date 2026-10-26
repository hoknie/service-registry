package types

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/google/uuid"
)

type NodeSettings struct {
	IncludeSet    bool
	Include       []string
	Exclude       *[]string
	Refs          *[]RefPattern
	APIs          *bool
	APIPaths      *[]string
	Refresh       *Refresh
	RefreshEvents *[]CollectEvent
}

func (n NodeSettings) IsEmpty() bool {
	return !n.IncludeSet && n.Exclude == nil && n.Refs == nil && n.APIs == nil && n.APIPaths == nil && n.Refresh == nil &&
		n.RefreshEvents == nil
}

func (n NodeSettings) Stored() NodeSettings {
	if !n.IncludeSet {
		n.Include = nil
	}
	return n
}

type ChainNode struct {
	ID   uuid.UUID
	Kind string
	Name string
	Own  NodeSettings
}

type From struct{ Include, Exclude, Refs, APIs, APIPaths, Refresh, RefreshEvents int }

func Merge(chain []ChainNode) (Settings, From) {
	s, from := DefaultSettings(), From{Include: -1, Exclude: -1, Refs: -1, APIs: -1, APIPaths: -1, Refresh: -1, RefreshEvents: -1}
	for i := len(chain) - 1; i >= 0; i-- {
		own := chain[i].Own
		if own.IncludeSet {
			s.Include, from.Include = own.Include, i
		}
		if own.Exclude != nil {
			s.Exclude, from.Exclude = *own.Exclude, i
		}
		if own.Refs != nil {
			s.Refs, from.Refs = *own.Refs, i
		}
		if own.APIs != nil {
			s.APIs, from.APIs = *own.APIs, i
		}
		if own.APIPaths != nil {
			s.APIPaths, from.APIPaths = *own.APIPaths, i
		}
		if own.Refresh != nil {
			s.Refresh, from.Refresh = *own.Refresh, i
		}
		if own.RefreshEvents != nil {
			s.RefreshEvents, from.RefreshEvents = *own.RefreshEvents, i
		}
	}
	return s, from
}

func ValidateNodeSettings(n NodeSettings) (NodeSettings, error) {
	if n.Refresh != nil && !n.Refresh.Valid() {
		return NodeSettings{}, InvalidSettings
	}
	if n.RefreshEvents != nil && !validEvents(*n.RefreshEvents) {
		return NodeSettings{}, InvalidSettings
	}
	if n.Refs != nil && !validRefs(*n.Refs) {
		return NodeSettings{}, InvalidSettings
	}
	lists := [][]string{n.Include}
	for _, list := range []*[]string{n.Exclude, n.APIPaths} {
		if list != nil {
			lists = append(lists, *list)
		}
	}
	for _, list := range lists {
		if !validPatterns(list) {
			return NodeSettings{}, InvalidSettings
		}
	}
	return n, nil
}

func validEvents(events []CollectEvent) bool {
	seen := map[CollectEvent]bool{}
	for _, e := range events {
		if !e.Valid() || seen[e] {
			return false
		}
		seen[e] = true
	}
	return true
}

type RefKind string

const (
	RefBranch RefKind = "branch"
	RefTag    RefKind = "tag"
)

func (k RefKind) Valid() bool { return k == RefBranch || k == RefTag }

type RefPattern struct {
	Pattern string
	Kind    RefKind
	Release bool
}

func validRefs(refs []RefPattern) bool {
	if len(refs) > maxPatterns {
		return false
	}
	seen := map[RefPattern]bool{}
	for _, r := range refs {
		key := RefPattern{Pattern: r.Pattern, Kind: r.Kind}
		if !r.Kind.Valid() || (r.Release && r.Kind != RefTag) || seen[key] || !validPatterns([]string{r.Pattern}) {
			return false
		}
		seen[key] = true
	}
	return true
}

func matchRefs(refs []RefPattern, kind RefKind, releaseOnly bool, name string) bool {
	for _, r := range refs {
		if r.Kind != kind || (releaseOnly && !r.Release) {
			continue
		}
		if ok, _ := doublestar.Match(r.Pattern, name); ok {
			return true
		}
	}
	return false
}

type TagInfo struct {
	Name      string
	FirstSeen time.Time
}

type Tag struct {
	Name      string
	Commit    string
	FirstSeen time.Time
}

type StoredTag struct {
	Tag
	Release bool
}

func SortTags[T any](tags []T, info func(T) TagInfo) {
	slices.SortStableFunc(tags, func(a, b T) int { return CompareTags(info(b), info(a)) })
}

func TagVersion(name string) []uint64 {
	start := strings.IndexAny(name, "0123456789")
	if start < 0 {
		return nil
	}
	var out []uint64
	for _, part := range strings.Split(name[start:], ".") {
		end := 0
		for end < len(part) && part[end] >= '0' && part[end] <= '9' {
			end++
		}
		if end == 0 {
			break
		}
		n, err := strconv.ParseUint(part[:end], 10, 64)
		if err != nil {
			n = ^uint64(0)
		}
		out = append(out, n)
		if end < len(part) {
			break
		}
	}
	return out
}

func compareVersions(a, b []uint64) int {
	for i := range max(len(a), len(b)) {
		var x, y uint64
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func CompareTags(a, b TagInfo) int {
	va, vb := TagVersion(a.Name), TagVersion(b.Name)
	switch {
	case va != nil && vb == nil:
		return 1
	case va == nil && vb != nil:
		return -1
	case va != nil:
		if c := compareVersions(va, vb); c != 0 {
			return c
		}
	default:
		if c := a.FirstSeen.Compare(b.FirstSeen); c != 0 {
			return c
		}
	}
	return strings.Compare(a.Name, b.Name)
}

func PickTags(found map[string]string, known map[string]time.Time, s Settings, limit int, now time.Time) ([]Tag, string) {
	if limit <= 0 {
		return nil, ""
	}
	var all []Tag
	for name, commit := range found {
		if !s.CollectsTag(name) {
			continue
		}
		seen, ok := known[name]
		if !ok {
			seen = now
		}
		all = append(all, Tag{Name: name, Commit: commit, FirstSeen: seen})
	}
	SortTags(all, func(t Tag) TagInfo { return TagInfo{t.Name, t.FirstSeen} })
	release := ""
	for _, t := range all {
		if s.ReleaseTag(t.Name) {
			release = t.Name
			break
		}
	}
	picked := all[:min(limit, len(all))]
	if release != "" && !slices.ContainsFunc(picked, func(t Tag) bool { return t.Name == release }) {
		i := slices.IndexFunc(all, func(t Tag) bool { return t.Name == release })
		picked = append(slices.Clone(picked), all[i])
	}
	return picked, release
}
