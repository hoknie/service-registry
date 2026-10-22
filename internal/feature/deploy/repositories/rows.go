package repositories

import (
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
	ID           uuid.UUID          `db:"id"`
	Name         string             `db:"name"`
	Environment  string             `db:"environment"`
	InCluster    bool               `db:"in_cluster"`
	APIURL       *string            `db:"api_url"`
	CAPEM        *string            `db:"ca_pem"`
	SecretID     *uuid.UUID         `db:"credentials_secret_id"`
	HasEnc       bool               `db:"has_enc"`
	Ref          *string            `db:"credentials_ref"`
	Fingerprint  *string            `db:"credentials_fingerprint"`
	Namespaces   []string           `db:"namespaces"`
	Rules        []deploytypes.Rule `db:"rules"`
	IntervalSecs int32              `db:"interval_secs"`
	Enabled      bool               `db:"enabled"`
	Status       string             `db:"status"`
	LastError    *errorJSON         `db:"last_error"`
	LastPolledAt *string            `db:"last_polled_at"`
	NextRunAt    string             `db:"next_run_at"`
	Workloads    int64              `db:"workloads"`
	Unmatched    int64              `db:"unmatched"`
	CreatedAt    string             `db:"created_at"`
	UpdatedAt    string             `db:"updated_at"`
}

type errorJSON struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r clusterRow) cluster() deploytypes.Cluster {
	c := deploytypes.Cluster{
		ID: r.ID,
		Settings: deploytypes.Settings{Name: r.Name, Environment: r.Environment, InCluster: r.InCluster, APIURL: r.APIURL,
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
	switch {
	case r.InCluster:
	case r.SecretID != nil:
		c.Credentials = &catalogtypes.Credentials{Kind: catalogtypes.CredentialsSecret, SecretID: r.SecretID}
	default:
		var enc *string
		if r.HasEnc {
			enc = new(string)
		}
		creds := catalogtypes.LegacyCredentials(enc, r.Ref, r.Fingerprint)
		c.Credentials = &creds
	}
	return c
}

func Err(err error) error { return postgres.Translate(err, constraints) }

type previousRow struct {
	UID string
	deploytypes.Previous
}
