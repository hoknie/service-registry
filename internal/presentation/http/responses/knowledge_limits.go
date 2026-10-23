package responses

import knowledgetypes "svc-registry/internal/feature/knowledge/types"

type Limit struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

func limitsOf(list []knowledgetypes.LimitSetting) []Limit {
	if list == nil {
		return nil
	}
	out := make([]Limit, 0, len(list))
	for _, l := range list {
		out = append(out, Limit{Name: l.Name, Value: l.Value})
	}
	return out
}
