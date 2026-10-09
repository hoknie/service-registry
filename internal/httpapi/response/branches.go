package response

import "svc-registry/internal/catalog"

type Branch struct {
	Name           string   `json:"name"`
	HeadSHA        *string  `json:"head_sha"`
	IsDefault      bool     `json:"is_default"`
	Protected      *bool    `json:"protected"`
	Sources        []string `json:"sources"`
	Pinned         bool     `json:"pinned"`
	Stale          bool     `json:"stale"`
	FirstSeenAt    string   `json:"first_seen_at"`
	LastActivityAt string   `json:"last_activity_at"`
	GoneAt         *string  `json:"gone_at"`
}

func BranchOf(b catalog.Branch) Branch {
	sources := make([]string, 0, len(b.Sources))
	for _, s := range b.Sources {
		sources = append(sources, string(s))
	}
	return Branch{Name: b.Name, HeadSHA: b.HeadSHA, IsDefault: b.IsDefault, Protected: b.Protected, Sources: sources,
		Pinned: b.Pinned, Stale: b.Stale, FirstSeenAt: b.FirstSeenAt, LastActivityAt: b.LastActivityAt, GoneAt: b.GoneAt}
}

func BranchPage(items []catalog.Branch, f catalog.BranchFilter, total uint64) Page[Branch] {
	out := make([]Branch, 0, len(items))
	for _, b := range items {
		out = append(out, BranchOf(b))
	}
	return Page[Branch]{Items: out, Total: total, Limit: f.Limit, Offset: f.Offset}
}
