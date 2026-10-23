package responses

import (
	apisservices "svc-registry/internal/feature/apis/services"
	apistypes "svc-registry/internal/feature/apis/types"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

type APIList struct {
	Snapshot       *Snapshot           `json:"snapshot"`
	Items          []apistypes.Summary `json:"items"`
	HiddenVendored int                 `json:"hidden_vendored"`
}

func APIListOf(l apisservices.APIList) APIList {
	return APIList{Snapshot: SnapshotOf(&l.Snapshot), Items: l.Items, HiddenVendored: l.HiddenVendored}
}

type APIDocument struct {
	Snapshot *Snapshot `json:"snapshot"`
	apistypes.Document
}

func APIDocumentOf(s knowledgetypes.Snapshot, d apistypes.Document) APIDocument {
	return APIDocument{Snapshot: SnapshotOf(&s), Document: d}
}
