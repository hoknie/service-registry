package request

import "svc-registry/internal/knowledge"

type Scans struct {
	Page
	Project string `query:"project"`
	Kind    string `query:"kind"`
	Status  string `query:"status"`
	Trigger string `query:"trigger"`
}

func (q Scans) Filter() knowledge.ScanQuery {
	return knowledge.ScanQuery{Project: q.Project, Kind: q.Kind, Status: q.Status, Trigger: q.Trigger}
}
