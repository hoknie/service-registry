package types

import (
	"regexp"
	"strings"

	"github.com/google/uuid"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

type Version struct {
	Pattern string
	Kind    knowledgetypes.RefKind
}

type Settings struct {
	Publish        bool
	Versions       []Version
	DefaultBranch  bool
	CurrentRelease bool
}

func DefaultSettings() Settings {
	return Settings{Versions: []Version{}, DefaultBranch: true, CurrentRelease: true}
}

type NodeSettings struct {
	Publish        *bool
	Versions       *[]Version
	DefaultBranch  *bool
	CurrentRelease *bool
	Name           *string
	Host           *string
}

func (n NodeSettings) IsEmpty() bool {
	return n.Publish == nil && n.Versions == nil && n.DefaultBranch == nil && n.CurrentRelease == nil && n.Name == nil && n.Host == nil
}

type ChainNode struct {
	ID       uuid.UUID
	ParentID *uuid.UUID
	Kind     string
	Slug     string
	Name     string
	Own      NodeSettings
}

type From struct{ Publish, Versions, DefaultBranch, CurrentRelease int }

func (s Settings) Apply(own NodeSettings) Settings {
	if own.Publish != nil {
		s.Publish = *own.Publish
	}
	if own.Versions != nil {
		s.Versions = *own.Versions
	}
	if own.DefaultBranch != nil {
		s.DefaultBranch = *own.DefaultBranch
	}
	if own.CurrentRelease != nil {
		s.CurrentRelease = *own.CurrentRelease
	}
	return s
}

func Merge(chain []ChainNode) (Settings, From) {
	s, from := DefaultSettings(), From{-1, -1, -1, -1}
	for i := len(chain) - 1; i >= 0; i-- {
		own := chain[i].Own
		s = s.Apply(own)
		if own.Publish != nil {
			from.Publish = i
		}
		if own.Versions != nil {
			from.Versions = i
		}
		if own.DefaultBranch != nil {
			from.DefaultBranch = i
		}
		if own.CurrentRelease != nil {
			from.CurrentRelease = i
		}
	}
	return s, from
}

var (
	namePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	hostLabel   = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
)

const maxVersions = 50

func ValidHost(h string) bool {
	if h == "" || len(h) > 253 || h != strings.ToLower(h) {
		return false
	}
	for _, label := range strings.Split(h, ".") {
		if !hostLabel.MatchString(label) {
			return false
		}
	}
	return true
}

func Validate(n NodeSettings, kind, publicHost string) (NodeSettings, error) {
	if n.Name != nil {
		n.Name = new(strings.ToLower(*n.Name))
		if kind == "organization" || !namePattern.MatchString(*n.Name) {
			return NodeSettings{}, InvalidPublication
		}
	}
	if n.Host != nil && (!ValidHost(*n.Host) || *n.Host == publicHost) {
		return NodeSettings{}, InvalidPublication
	}
	if n.Versions != nil {
		refs := make([]knowledgetypes.RefPattern, len(*n.Versions))
		for i, v := range *n.Versions {
			refs[i] = knowledgetypes.RefPattern{Pattern: v.Pattern, Kind: v.Kind}
		}
		if len(refs) > maxVersions {
			return NodeSettings{}, InvalidPublication
		}
		if _, err := knowledgetypes.ValidateNodeSettings(knowledgetypes.NodeSettings{Refs: &refs}); err != nil {
			return NodeSettings{}, InvalidPublication
		}
	}
	return n, nil
}

func (s Settings) refs() []knowledgetypes.RefPattern {
	out := make([]knowledgetypes.RefPattern, len(s.Versions))
	for i, v := range s.Versions {
		out[i] = knowledgetypes.RefPattern{Pattern: v.Pattern, Kind: v.Kind}
	}
	return out
}

func (s Settings) VersionAllowed(kind knowledgetypes.RefKind, name string, isDefault, isRelease bool) bool {
	if kind == knowledgetypes.RefBranch && isDefault && s.DefaultBranch {
		return true
	}
	if kind == knowledgetypes.RefTag && isRelease && s.CurrentRelease {
		return true
	}
	k := knowledgetypes.Settings{Refs: s.refs()}
	if kind == knowledgetypes.RefTag {
		return k.CollectsTag(name)
	}
	return k.CollectsBranch(name, "")
}
