package deploy

import (
	"fmt"

	"svc-registry/internal/platform/apperr"
)

type Invalid int

const (
	InvalidEnvironmentKey Invalid = iota + 1
	InvalidEnvironmentNames
	InvalidPosition
	InvalidClusterName
	InvalidAPIURL
	InvalidCA
	InvalidNamespaces
	InvalidClusterRules
	InvalidPollInterval
)

func (i Invalid) Code() string {
	switch i {
	case InvalidEnvironmentKey:
		return "validation.invalid_environment_key"
	case InvalidEnvironmentNames:
		return "validation.invalid_environment_names"
	case InvalidPosition:
		return "validation.invalid_position"
	case InvalidClusterName:
		return "validation.invalid_cluster_name"
	case InvalidAPIURL:
		return "validation.invalid_api_url"
	case InvalidCA:
		return "validation.invalid_ca"
	case InvalidNamespaces:
		return "validation.invalid_namespaces"
	case InvalidClusterRules:
		return "validation.invalid_cluster_rules"
	case InvalidPollInterval:
		return "validation.invalid_poll_interval"
	}
	return "validation.invalid"
}

func (i Invalid) Message() string {
	switch i {
	case InvalidEnvironmentKey:
		return "environment must be 1 to 63 characters a-z, 0-9, '.', '_', '-', starting and ending with a letter or digit"
	case InvalidEnvironmentNames:
		return `names must have "en", "es", "ru" and "zh", each 1 to 100 characters without control characters`
	case InvalidPosition:
		return "position must be an integer 0..=10000"
	case InvalidClusterName:
		return "name must be 1 to 100 characters without control characters"
	case InvalidAPIURL:
		return "api_url must be an https URL with a host, required unless in_cluster"
	case InvalidCA:
		return "ca_pem must be PEM with at least one certificate"
	case InvalidNamespaces:
		return "namespaces must be at most 100 names of 1 to 63 characters a-z, 0-9, '-'"
	case InvalidClusterRules:
		return `rules must be at most 20 objects {"label", "project"} with a slug path template`
	case InvalidPollInterval:
		return "interval_secs must be an integer 15..=3600"
	}
	return "invalid input"
}

func (i Invalid) Error() string { return i.Message() }

type Conflict int

const (
	ConflictEnvironmentTaken Conflict = iota + 1
	ConflictClusterNameTaken
)

func (c Conflict) Code() string {
	switch c {
	case ConflictEnvironmentTaken:
		return "conflict.environment_taken"
	case ConflictClusterNameTaken:
		return "conflict.cluster_name_taken"
	}
	return "conflict.unknown"
}

func (c Conflict) Message() string {
	switch c {
	case ConflictEnvironmentTaken:
		return "an environment with this key already exists"
	case ConflictClusterNameTaken:
		return "a cluster with this name already exists"
	}
	return "conflict"
}

func (c Conflict) Error() string { return c.Message() }

type FailureCode string

const (
	FailUnreachable  FailureCode = "k8s.unreachable"
	FailTLS          FailureCode = "k8s.tls_error"
	FailUnauthorized FailureCode = "k8s.unauthorized"
	FailForbidden    FailureCode = "k8s.forbidden"
	FailTimeout      FailureCode = "k8s.timeout"
)

type Failure struct {
	Code    FailureCode
	Message string
}

func (f *Failure) Error() string { return fmt.Sprintf("%s: %s", f.Code, f.Message) }

var (
	ErrNotFound    = apperr.Sentinel(apperr.NotFound, "not found")
	ErrUnavailable = apperr.Sentinel(apperr.Unavailable, "database is unavailable")
)

type InternalError struct{ Detail string }

func (e *InternalError) Error() string { return "internal: " + e.Detail }

func (i Invalid) AppError() *apperr.Error { return apperr.Invalid(i.Code(), i.Message()) }

func (c Conflict) AppError() *apperr.Error { return apperr.Conflicting(c.Code(), c.Message()) }

func (e *InternalError) AppError() *apperr.Error { return apperr.InternalDetail(e.Detail) }
