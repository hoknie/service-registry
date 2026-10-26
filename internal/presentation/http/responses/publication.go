package responses

import (
	publishingservices "svc-registry/internal/feature/publishing/services"
	publishingtypes "svc-registry/internal/feature/publishing/types"
)

type PublicationVersion struct {
	Pattern string `json:"pattern"`
	Kind    string `json:"kind"`
}

type PublicationEntry struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

type Publication struct {
	Publish        bool                       `json:"publish"`
	Versions       []PublicationVersion       `json:"versions"`
	DefaultBranch  bool                       `json:"default_branch"`
	CurrentRelease bool                       `json:"current_release"`
	Name           *string                    `json:"name"`
	Host           *string                    `json:"host"`
	Own            map[string]any             `json:"own"`
	From           map[string]*SettingsOrigin `json:"from"`
	Entries        []PublicationEntry         `json:"entries"`
	CanEdit        bool                       `json:"can_edit"`
}

func versionsOf(vs []publishingtypes.Version) []PublicationVersion {
	out := make([]PublicationVersion, len(vs))
	for i, v := range vs {
		out[i] = PublicationVersion{Pattern: v.Pattern, Kind: string(v.Kind)}
	}
	return out
}

func PublicationOf(p publishingservices.Publication) Publication {
	own := map[string]any{}
	if p.Own.Publish != nil {
		own["publish"] = *p.Own.Publish
	}
	if p.Own.Versions != nil {
		own["versions"] = versionsOf(*p.Own.Versions)
	}
	if p.Own.DefaultBranch != nil {
		own["default_branch"] = *p.Own.DefaultBranch
	}
	if p.Own.CurrentRelease != nil {
		own["current_release"] = *p.Own.CurrentRelease
	}
	if p.Own.Name != nil {
		own["name"] = *p.Own.Name
	}
	if p.Own.Host != nil {
		own["host"] = *p.Own.Host
	}
	from := map[string]*SettingsOrigin{}
	for k, n := range p.From {
		if n == nil {
			from[k] = nil
			continue
		}
		from[k] = &SettingsOrigin{ID: n.ID, Kind: n.Kind, Name: n.Name}
	}
	out := Publication{Publish: p.Settings.Publish, Versions: versionsOf(p.Settings.Versions), DefaultBranch: p.Settings.DefaultBranch,
		CurrentRelease: p.Settings.CurrentRelease, Name: p.Own.Name, Host: p.Own.Host, Own: own, From: from,
		Entries: make([]PublicationEntry, 0, len(p.Entries)), CanEdit: p.CanEdit}
	for _, e := range p.Entries {
		out.Entries = append(out.Entries, PublicationEntry{Kind: string(e.Kind), URL: e.URL})
	}
	return out
}
