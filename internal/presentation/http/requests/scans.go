package requests

import knowledgetypes "svc-registry/internal/feature/knowledge/types"

type Scans struct {
	Page
	Project string `query:"project"`
	Kind    string `query:"kind"`
	Status  string `query:"status"`
	Trigger string `query:"trigger"`
}

func (q Scans) Filter() knowledgetypes.ScanQuery {
	return knowledgetypes.ScanQuery{Project: q.Project, Kind: q.Kind, Status: q.Status, Trigger: q.Trigger}
}
