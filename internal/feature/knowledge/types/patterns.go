package types

import "github.com/google/uuid"

type NodeSettings struct {
	IncludeSet bool
	Include    []string
	Exclude    *[]string
	Branches   *[]string
	APIs       *bool
	Refresh    *Refresh
}

func (n NodeSettings) IsEmpty() bool {
	return !n.IncludeSet && n.Exclude == nil && n.Branches == nil && n.APIs == nil && n.Refresh == nil
}

type ChainNode struct {
	ID   uuid.UUID
	Kind string
	Name string
	Own  NodeSettings
}

type From struct{ Include, Exclude, Branches, APIs, Refresh int }

func Merge(chain []ChainNode) (Settings, From) {
	s, from := DefaultSettings(), From{Include: -1, Exclude: -1, Branches: -1, APIs: -1, Refresh: -1}
	for i := len(chain) - 1; i >= 0; i-- {
		own := chain[i].Own
		if own.IncludeSet {
			s.Include, from.Include = own.Include, i
		}
		if own.Exclude != nil {
			s.Exclude, from.Exclude = *own.Exclude, i
		}
		if own.Branches != nil {
			s.Branches, from.Branches = *own.Branches, i
		}
		if own.APIs != nil {
			s.APIs, from.APIs = *own.APIs, i
		}
		if own.Refresh != nil {
			s.Refresh, from.Refresh = *own.Refresh, i
		}
	}
	return s, from
}

func ValidateNodeSettings(n NodeSettings) (NodeSettings, error) {
	if n.Refresh != nil && !n.Refresh.Valid() {
		return NodeSettings{}, InvalidSettings
	}
	lists := [][]string{n.Include}
	if n.Exclude != nil {
		lists = append(lists, *n.Exclude)
	}
	if n.Branches != nil {
		lists = append(lists, *n.Branches)
	}
	for _, list := range lists {
		if !validPatterns(list) {
			return NodeSettings{}, InvalidSettings
		}
	}
	return n, nil
}
