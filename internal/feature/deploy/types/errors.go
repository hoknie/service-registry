package types

import (
	"fmt"

	"svc-registry/internal/platform/apperr"
)

var (
	InvalidEnvironmentKey   = apperr.Invalid("validation.invalid_environment_key", "environment must be 1 to 63 characters a-z, 0-9, '.', '_', '-', starting and ending with a letter or digit")
	InvalidEnvironmentNames = apperr.Invalid("validation.invalid_environment_names", `names must have "en", "es", "ru" and "zh", each 1 to 100 characters without control characters`)
	InvalidPosition         = apperr.Invalid("validation.invalid_position", "position must be an integer 0..=10000")
	InvalidClusterName      = apperr.Invalid("validation.invalid_cluster_name", "name must be 1 to 100 characters without control characters")
	InvalidAPIURL           = apperr.Invalid("validation.invalid_api_url", "api_url must be an https URL with a host, required unless in_cluster")
	InvalidCA               = apperr.Invalid("validation.invalid_ca", "ca_pem must be PEM with at least one certificate")
	InvalidNamespaces       = apperr.Invalid("validation.invalid_namespaces", "namespaces must be at most 100 names of 1 to 63 characters a-z, 0-9, '-'")
	InvalidClusterRules     = apperr.Invalid("validation.invalid_cluster_rules", `rules must be at most 20 objects {"label", "project"} with a slug path template`)
	InvalidPollInterval     = apperr.Invalid("validation.invalid_poll_interval", "interval_secs must be an integer 15..=3600")
)

var (
	ConflictEnvironmentTaken = apperr.Conflicting("conflict.environment_taken", "an environment with this key already exists")
	ConflictClusterNameTaken = apperr.Conflicting("conflict.cluster_name_taken", "a cluster with this name already exists")
)

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
