package responses

import (
	"github.com/google/uuid"

	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

type KnowledgeSettings struct {
	Include       []string     `json:"include"`
	Exclude       []string     `json:"exclude"`
	Refs          []RefPattern `json:"refs"`
	APIs          bool         `json:"apis"`
	APIPaths      []string     `json:"api_paths"`
	Refresh       string       `json:"refresh"`
	RefreshEvents []string     `json:"refresh_events"`
}

type NodeSettings struct {
	KnowledgeSettings
	IntervalSecs uint32                     `json:"interval_secs"`
	MaxTags      uint32                     `json:"max_tags"`
	Defaults     SettingsDefaults           `json:"defaults"`
	Own          map[string]any             `json:"own"`
	From         map[string]*SettingsOrigin `json:"from"`
}

type SettingsDefaults struct {
	APIPaths      []string `json:"api_paths"`
	RefreshEvents []string `json:"refresh_events"`
}

type SettingsOrigin struct {
	ID   uuid.UUID `json:"id"`
	Kind string    `json:"kind"`
	Name string    `json:"name"`
}

func NodeSettingsOf(s knowledgeservices.KnowledgeSettings) NodeSettings {
	own := map[string]any{}
	if s.Own.IncludeSet {
		own["include"] = s.Own.Include
	}
	if s.Own.Exclude != nil {
		own["exclude"] = *s.Own.Exclude
	}
	if s.Own.Refs != nil {
		own["refs"] = refPatterns(*s.Own.Refs)
	}
	if s.Own.APIs != nil {
		own["apis"] = *s.Own.APIs
	}
	if s.Own.APIPaths != nil {
		own["api_paths"] = nonNil(*s.Own.APIPaths)
	}
	if s.Own.Refresh != nil {
		own["refresh"] = *s.Own.Refresh
	}
	if s.Own.RefreshEvents != nil {
		own["refresh_events"] = eventNames(*s.Own.RefreshEvents)
	}
	origin := func(n *knowledgetypes.ChainNode) *SettingsOrigin {
		if n == nil {
			return nil
		}
		return &SettingsOrigin{ID: n.ID, Kind: n.Kind, Name: n.Name}
	}
	return NodeSettings{KnowledgeSettings: KnowledgeSettingsOf(s.Settings), IntervalSecs: s.IntervalSecs, MaxTags: s.MaxTags,
		Defaults: SettingsDefaults{APIPaths: nonNil(s.Defaults.APIPaths), RefreshEvents: eventNames(s.Defaults.RefreshEvents)}, Own: own,
		From: map[string]*SettingsOrigin{"include": origin(s.IncludeFrom), "exclude": origin(s.ExcludeFrom), "refs": origin(s.RefsFrom),
			"apis": origin(s.APIsFrom), "api_paths": origin(s.APIPathsFrom), "refresh": origin(s.RefreshFrom),
			"refresh_events": origin(s.RefreshEventsFrom)}}
}

func KnowledgeSettingsOf(s knowledgetypes.Settings) KnowledgeSettings {
	return KnowledgeSettings{Include: s.Include, Exclude: nonNil(s.Exclude), Refs: refPatterns(s.Refs), APIs: s.APIs,
		APIPaths: nonNil(s.APIPaths), Refresh: string(s.Refresh), RefreshEvents: eventNames(s.RefreshEvents)}
}

func eventNames(events []knowledgetypes.CollectEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = string(e)
	}
	return out
}

type RefPattern struct {
	Pattern string `json:"pattern"`
	Kind    string `json:"kind"`
	Release bool   `json:"release"`
}

func refPatterns(refs []knowledgetypes.RefPattern) []RefPattern {
	out := make([]RefPattern, len(refs))
	for i, r := range refs {
		out[i] = RefPattern{Pattern: r.Pattern, Kind: string(r.Kind), Release: r.Release}
	}
	return out
}
