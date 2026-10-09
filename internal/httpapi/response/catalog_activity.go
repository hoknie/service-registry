package response

import "svc-registry/internal/catalog"

type Process struct {
	Kind    string  `json:"kind"`
	State   string  `json:"state"`
	Code    *string `json:"code"`
	LastAt  *string `json:"last_at"`
	Pending *int64  `json:"pending"`
}

type Summary struct {
	Running int64 `json:"running"`
	Queued  int64 `json:"queued"`
	Failed  int64 `json:"failed"`
}

type Activity struct {
	Processes *[]Process `json:"processes,omitempty"`
	Summary   *Summary   `json:"summary,omitempty"`
}

func ProcessesOf(processes []catalog.Process) []Process {
	out := make([]Process, 0, len(processes))
	for _, p := range processes {
		out = append(out, Process{Kind: string(p.Kind), State: string(p.State), Code: p.Code, LastAt: p.LastAt, Pending: p.Pending})
	}
	return out
}

func SummaryOf(s catalog.Summary) Summary {
	return Summary{Running: s.Running, Queued: s.Queued, Failed: s.Failed}
}

func ActivityOf(a catalog.Activity) Activity {
	if a.Project {
		p := ProcessesOf(a.Processes)
		return Activity{Processes: &p}
	}
	s := SummaryOf(a.Summary)
	return Activity{Summary: &s}
}
