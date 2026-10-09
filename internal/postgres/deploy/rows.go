package deploy

import (
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"svc-registry/internal/apperr"
	domain "svc-registry/internal/deploy"
	"svc-registry/internal/forge"
)

type environmentRow struct {
	ID        uuid.UUID         `db:"id"`
	Key       string            `db:"key"`
	Names     map[string]string `db:"names"`
	Position  int32             `db:"position"`
	CreatedAt string            `db:"created_at"`
	UpdatedAt string            `db:"updated_at"`
}

func (r environmentRow) environment() domain.Environment {
	return domain.Environment{ID: r.ID, Key: r.Key, Names: r.Names, Position: r.Position, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func dbErr(err error) error {
	if err == nil {
		return nil
	}
	var invalid domain.Invalid
	var conflict domain.Conflict
	var internalErr *domain.InternalError
	if errors.As(err, &invalid) || errors.As(err, &conflict) || errors.Is(err, domain.ErrNotFound) || errors.As(err, &internalErr) {
		return err
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.ConstraintName {
		case "environments_key_key":
			return domain.ConflictEnvironmentTaken
		case "clusters_name_key":
			return domain.ConflictClusterNameTaken
		}
	}
	if apperr.IsUnavailable(err) {
		return domain.ErrUnavailable
	}
	return &domain.InternalError{Detail: err.Error()}
}

type clusterRow struct {
	ID           uuid.UUID     `db:"id"`
	Name         string        `db:"name"`
	Environment  string        `db:"environment"`
	InCluster    bool          `db:"in_cluster"`
	APIURL       *string       `db:"api_url"`
	CAPEM        *string       `db:"ca_pem"`
	Ref          *string       `db:"credentials_ref"`
	Fingerprint  *string       `db:"credentials_fingerprint"`
	Namespaces   []string      `db:"namespaces"`
	Rules        []domain.Rule `db:"rules"`
	IntervalSecs int32         `db:"interval_secs"`
	Enabled      bool          `db:"enabled"`
	Status       string        `db:"status"`
	LastError    *errorJSON    `db:"last_error"`
	LastPolledAt *string       `db:"last_polled_at"`
	NextRunAt    string        `db:"next_run_at"`
	Workloads    int64         `db:"workloads"`
	Unmatched    int64         `db:"unmatched"`
	CreatedAt    string        `db:"created_at"`
	UpdatedAt    string        `db:"updated_at"`
}

type errorJSON struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r clusterRow) cluster() domain.Cluster {
	c := domain.Cluster{
		ID: r.ID,
		Settings: domain.Settings{Name: r.Name, Environment: r.Environment, InCluster: r.InCluster, APIURL: r.APIURL,
			CAPEM: r.CAPEM, Namespaces: r.Namespaces, Rules: r.Rules, IntervalSecs: r.IntervalSecs, Enabled: r.Enabled},
		Status: domain.Status(r.Status), LastPolledAt: r.LastPolledAt, NextPollAt: r.NextRunAt,
		Workloads: r.Workloads, Unmatched: r.Unmatched, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if c.Namespaces == nil {
		c.Namespaces = []string{}
	}
	if c.Rules == nil {
		c.Rules = []domain.Rule{}
	}
	if r.LastError != nil {
		c.LastError = &domain.Failure{Code: domain.FailureCode(r.LastError.Code), Message: r.LastError.Message}
	}
	switch {
	case r.InCluster:
	case r.Ref != nil:
		c.Credentials = &forge.Credentials{Kind: forge.CredentialsRef, Ref: *r.Ref}
	default:
		c.Credentials = &forge.Credentials{Kind: forge.CredentialsToken, Fingerprint: str(r.Fingerprint)}
	}
	return c
}

func str(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
