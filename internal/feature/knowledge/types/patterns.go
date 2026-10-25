package types

import "github.com/google/uuid"

type NodeSettings struct {
	IncludeSet    bool
	Include       []string
	Exclude       *[]string
	Branches      *[]string
	APIs          *bool
	APIPaths      *[]string
	Refresh       *Refresh
	RefreshEvents *[]CollectEvent
}

func (n NodeSettings) IsEmpty() bool {
	return !n.IncludeSet && n.Exclude == nil && n.Branches == nil && n.APIs == nil && n.APIPaths == nil && n.Refresh == nil &&
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

type From struct{ Include, Exclude, Branches, APIs, APIPaths, Refresh, RefreshEvents int }

func Merge(chain []ChainNode) (Settings, From) {
	s, from := DefaultSettings(), From{Include: -1, Exclude: -1, Branches: -1, APIs: -1, APIPaths: -1, Refresh: -1, RefreshEvents: -1}
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
	lists := [][]string{n.Include}
	for _, list := range []*[]string{n.Exclude, n.Branches, n.APIPaths} {
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
