package repositories

import (
	"time"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/postgres"
)

type environmentRow struct {
	ID        uuid.UUID         `db:"id"`
	Key       string            `db:"key"`
	Names     map[string]string `db:"names"`
	Position  int32             `db:"position"`
	CreatedAt string            `db:"created_at"`
	UpdatedAt string            `db:"updated_at"`
}

func (r environmentRow) environment() deploytypes.Environment { return deploytypes.Environment(r) }

type clusterRow struct {
	ID            uuid.UUID                  `db:"id"`
	Mode          string                     `db:"mode"`
	Name          string                     `db:"name"`
	Environment   string                     `db:"environment"`
	InCluster     bool                       `db:"in_cluster"`
	APIURL        *string                    `db:"api_url"`
	CAPEM         *string                    `db:"ca_pem"`
	SecretID      *uuid.UUID                 `db:"credentials_secret_id"`
	Namespaces    []string                   `db:"namespaces"`
	Rules         []deploytypes.Rule         `db:"rules"`
	IntervalSecs  int32                      `db:"interval_secs"`
	Enabled       bool                       `db:"enabled"`
	Status        string                     `db:"status"`
	LastError     *errorJSON                 `db:"last_error"`
	LastPolledAt  *string                    `db:"last_polled_at"`
	NextRunAt     string                     `db:"next_run_at"`
	AgentVersion  *string                    `db:"agent_version"`
	ReportedAt    *string                    `db:"agent_reported_at"`
	AgentReport   *deploytypes.ReportSummary `db:"agent_report"`
	ReportGapSecs int32                      `db:"report_gap_secs"`
	AgentLoads    int32                      `db:"agent_workloads"`
	TokenPrefixes []string                   `db:"token_prefixes"`
	Workloads     int64                      `db:"workloads"`
	Unmatched     int64                      `db:"unmatched"`
	CreatedAt     string                     `db:"created_at"`
	UpdatedAt     string                     `db:"updated_at"`
}

type errorJSON struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r clusterRow) cluster() deploytypes.Cluster {
	c := deploytypes.Cluster{
		ID: r.ID,
		Settings: deploytypes.Settings{Mode: deploytypes.Mode(r.Mode), Name: r.Name, Environment: r.Environment, InCluster: r.InCluster, APIURL: r.APIURL,
			CAPEM: r.CAPEM, Namespaces: r.Namespaces, Rules: r.Rules, IntervalSecs: r.IntervalSecs, Enabled: r.Enabled},
		Status: deploytypes.Status(r.Status), LastPolledAt: r.LastPolledAt, NextPollAt: r.NextRunAt,
		Workloads: r.Workloads, Unmatched: r.Unmatched, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if c.Namespaces == nil {
		c.Namespaces = []string{}
	}
	if c.Rules == nil {
		c.Rules = []deploytypes.Rule{}
	}
	if r.LastError != nil {
		c.LastError = &deploytypes.Failure{Code: deploytypes.FailureCode(r.LastError.Code), Message: r.LastError.Message}
	}
	if c.Mode == deploytypes.ModeAgent {
		c.Agent = &deploytypes.AgentState{Version: r.AgentVersion, LastReportAt: r.ReportedAt, TokenPrefixes: r.TokenPrefixes,
			Report: r.AgentReport, ReportGapSecs: r.ReportGapSecs, ReportWorkloads: r.AgentLoads}
		if c.Agent.TokenPrefixes == nil {
			c.Agent.TokenPrefixes = []string{}
		}
	}
	switch {
	case r.InCluster:
	case r.SecretID != nil:
		c.Credentials = &catalogtypes.Credentials{Kind: catalogtypes.CredentialsSecret, SecretID: r.SecretID}
	}
	return c
}

func Err(err error) error { return postgres.Translate(err, constraints) }

type previousRow struct {
	UID string
	deploytypes.Previous
}

type workloadWrite struct {
	ID             uuid.UUID           `json:"id"`
	UID            string              `json:"uid"`
	Namespace      string              `json:"namespace"`
	Kind           string              `json:"kind"`
	Name           string              `json:"name"`
	ProjectID      *uuid.UUID          `json:"project_id"`
	Reason         *string             `json:"reason"`
	Annotation     *string             `json:"annotation"`
	Service        *string             `json:"service"`
	Environment    *string             `json:"environment"`
	Branch         *string             `json:"branch"`
	Version        *string             `json:"version"`
	Images         []deploytypes.Image `json:"images"`
	State          deploytypes.State   `json:"state"`
	PendingVersion *string             `json:"pending_version"`
	PendingSince   *time.Time          `json:"pending_since"`
}
