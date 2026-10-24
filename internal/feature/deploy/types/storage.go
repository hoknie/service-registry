package types

import (
	"strings"

	"github.com/google/uuid"
)

type LegacyCluster struct {
	ID   uuid.UUID
	Name string
	Enc  *string
	Ref  *string
}

type ProjectRef struct {
	ID      uuid.UUID
	Observe bool
}

type PathNode struct {
	ProjectRef
	Project bool
}

func ProjectPath(ref string) (*uuid.UUID, []string) {
	if id, err := uuid.Parse(ref); err == nil {
		return &id, nil
	}
	segments := strings.Split(ref, "/")
	for i, seg := range segments {
		segments[i] = strings.ToLower(seg)
	}
	return nil, segments
}

type StoredCredentials struct {
	SecretID *uuid.UUID
}

type NewAgentToken struct {
	ID        uuid.UUID
	ClusterID uuid.UUID
	Prefix    string
	Hash      [32]byte
}

type AgentToken struct {
	ID        uuid.UUID
	Prefix    string
	CreatedAt string
	ExpiresAt *string
}

type AgentTokenUse struct {
	ID    uuid.UUID
	Stale bool
}

func (s Settings) Stored() Settings {
	if s.Namespaces == nil {
		s.Namespaces = []string{}
	}
	if s.Rules == nil {
		s.Rules = []Rule{}
	}
	return s
}

func StatusOf(f *Failure) Status {
	if f != nil {
		return StatusError
	}
	return StatusOK
}

type StoredWorkload struct {
	ID uuid.UUID
	Observation
}

func StoredWorkloads(obs []Observation, newID func() uuid.UUID) ([]StoredWorkload, []string) {
	last := make(map[string]int, len(obs))
	for i, o := range obs {
		last[o.UID] = i
	}
	rows := make([]StoredWorkload, 0, len(last))
	uids := make([]string, 0, len(last))
	for i, o := range obs {
		if last[o.UID] != i {
			continue
		}
		if o.Images == nil {
			o.Images = []Image{}
		}
		uids = append(uids, o.UID)
		rows = append(rows, StoredWorkload{ID: newID(), Observation: o})
	}
	return rows, uids
}

type FinishedReport struct {
	Status    Status
	Failure   *Failure
	Version   string
	Workloads int
	Summary   ReportSummary
}

func FinishedReportOf(version string, workloads int, summary ReportSummary) FinishedReport {
	r := FinishedReport{Version: version, Workloads: workloads, Summary: summary}
	if e := summary.Error; e != nil {
		r.Failure = &Failure{Code: FailureCode(e.Code), Message: e.Message}
	}
	r.Status = StatusOf(r.Failure)
	return r
}
