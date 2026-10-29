package responses

import (
	pollingservices "svc-registry/internal/feature/polling/services"
	pollingtypes "svc-registry/internal/feature/polling/types"
)

type Polling struct {
	IntervalSecs *int                       `json:"interval_secs"`
	TimeoutSecs  int                        `json:"timeout_secs"`
	Mode         string                     `json:"mode"`
	Own          map[string]any             `json:"own"`
	From         map[string]*SettingsOrigin `json:"from"`
	Defaults     PollingDefaults            `json:"defaults"`
}

type PollingDefaults struct {
	RepoSyncIntervalSecs int `json:"repo_sync_interval_secs"`
	DocsIntervalSecs     int `json:"docs_interval_secs"`
	SkillsIntervalSecs   int `json:"skills_interval_secs"`
	TimeoutSecs          int `json:"timeout_secs"`
}

func PollingOf(p pollingservices.Polling) Polling {
	own := map[string]any{}
	if p.Own.IntervalSecs != nil {
		own["interval_secs"] = *p.Own.IntervalSecs
	}
	if p.Own.TimeoutSecs != nil {
		own["timeout_secs"] = *p.Own.TimeoutSecs
	}
	if p.Own.Mode != nil {
		own["mode"] = string(*p.Own.Mode)
	}
	from := map[string]*SettingsOrigin{}
	for k, n := range p.From {
		from[k] = originOf(n)
	}
	d := p.Defaults
	return Polling{IntervalSecs: p.Effective.IntervalSecs, TimeoutSecs: p.Effective.TimeoutSecs, Mode: string(p.Effective.Mode),
		Own: own, From: from, Defaults: PollingDefaults{RepoSyncIntervalSecs: d.RepoSyncIntervalSecs, DocsIntervalSecs: d.DocsIntervalSecs,
			SkillsIntervalSecs: d.SkillsIntervalSecs, TimeoutSecs: d.TimeoutSecs}}
}

func originOf(n *pollingtypes.ChainNode) *SettingsOrigin {
	if n == nil {
		return nil
	}
	return &SettingsOrigin{ID: n.ID, Kind: n.Kind, Name: n.Name}
}
