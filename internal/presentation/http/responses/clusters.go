package responses

import (
	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/deploy"
	deployservice "svc-registry/internal/feature/deploy/service"
)

type ClusterError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func clusterError(f *deploy.Failure) *ClusterError {
	if f == nil {
		return nil
	}
	return &ClusterError{Code: string(f.Code), Message: f.Message}
}

type Cluster struct {
	ID           uuid.UUID     `json:"id"`
	Name         string        `json:"name"`
	Environment  string        `json:"environment"`
	InCluster    bool          `json:"in_cluster"`
	APIURL       *string       `json:"api_url"`
	CAPEM        *string       `json:"ca_pem"`
	Credentials  *Credentials  `json:"credentials"`
	Namespaces   []string      `json:"namespaces"`
	Rules        []deploy.Rule `json:"rules"`
	IntervalSecs int32         `json:"interval_secs"`
	Enabled      bool          `json:"enabled"`
	Status       string        `json:"status"`
	LastError    *ClusterError `json:"last_error"`
	LastPolledAt *string       `json:"last_polled_at"`
	NextPollAt   string        `json:"next_poll_at"`
	Workloads    int64         `json:"workloads"`
	Unmatched    int64         `json:"unmatched"`
	CreatedAt    string        `json:"created_at"`
	UpdatedAt    string        `json:"updated_at"`
}

func ClusterOf(c deploy.Cluster) Cluster {
	out := Cluster{ID: c.ID, Name: c.Name, Environment: c.Environment, InCluster: c.InCluster, APIURL: c.APIURL,
		CAPEM: c.CAPEM, Namespaces: c.Namespaces, Rules: c.Rules, IntervalSecs: c.IntervalSecs, Enabled: c.Enabled,
		Status: string(c.Status), LastError: clusterError(c.LastError), LastPolledAt: c.LastPolledAt, NextPollAt: c.NextPollAt,
		Workloads: c.Workloads, Unmatched: c.Unmatched, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
	if c.Credentials != nil {
		creds := CredentialsOf(*c.Credentials)
		out.Credentials = &creds
	}
	return out
}

type ClusterTest struct {
	OK      bool          `json:"ok"`
	Version *string       `json:"version"`
	Missing []string      `json:"missing"`
	Error   *ClusterError `json:"error,omitempty"`
}

func ClusterTestOf(t deployservice.ClusterTest) ClusterTest {
	missing := t.Missing
	if missing == nil {
		missing = []string{}
	}
	return ClusterTest{OK: t.OK, Version: t.Version, Missing: missing, Error: clusterError(t.Error)}
}

type UnmatchedWorkload struct {
	Namespace  string  `json:"namespace"`
	Kind       string  `json:"kind"`
	Name       string  `json:"name"`
	Reason     string  `json:"reason"`
	Annotation *string `json:"annotation"`
	ObservedAt string  `json:"observed_at"`
}

func UnmatchedPage(p access.Page[deploy.Unmatched]) Page[UnmatchedWorkload] {
	return PageOf(p, func(u deploy.Unmatched) UnmatchedWorkload {
		return UnmatchedWorkload{Namespace: u.Namespace, Kind: string(u.Kind), Name: u.Name, Reason: string(u.Reason),
			Annotation: u.Annotation, ObservedAt: u.ObservedAt}
	})
}
