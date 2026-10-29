package requests

import (
	"bytes"
	"encoding/json"

	pollingtypes "svc-registry/internal/feature/polling/types"
)

type Polling struct {
	IntervalSecs *int    `json:"interval_secs"`
	TimeoutSecs  *int    `json:"timeout_secs"`
	Mode         *string `json:"mode"`
}

func ParsePolling(body []byte) (pollingtypes.NodeSettings, error) {
	var in Polling
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return pollingtypes.NodeSettings{}, pollingtypes.InvalidPolling
	}
	out := pollingtypes.NodeSettings{IntervalSecs: in.IntervalSecs, TimeoutSecs: in.TimeoutSecs}
	if in.Mode != nil {
		out.Mode = new(pollingtypes.Mode(*in.Mode))
	}
	return out, nil
}
