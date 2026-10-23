package types

import (
	"errors"
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
	InvalidClusterMode      = apperr.Invalid("validation.invalid_cluster_mode", `mode must be "poll" or "agent"; in_cluster needs the mode "poll"`)
	InvalidReport           = apperr.Invalid("validation.invalid_body", "the report does not match the agent report format")
)

var (
	ConflictEnvironmentTaken = apperr.Conflicting("conflict.environment_taken", "an environment with this key already exists")
	ConflictClusterNameTaken = apperr.Conflicting("conflict.cluster_name_taken", "a cluster with this name already exists")
	ConflictClusterModeAgent = apperr.Conflicting("conflict.cluster_mode_agent", "the agent in the cluster sends its observations; the registry does not poll it")
	ConflictClusterModePoll  = apperr.Conflicting("conflict.cluster_mode_poll", "the registry polls this cluster; it has no agent tokens")
	ConflictClusterDisabled  = apperr.Conflicting("conflict.cluster_disabled", "the cluster is disabled")
	ConflictReportInProgress = apperr.Conflicting("conflict.agent_report_in_progress", "a report of this cluster is being processed")
)

var (
	ErrNotInCluster   = errors.New("not running in a Kubernetes pod")
	ErrAgentRefused   = errors.New("the registry refused the agent token")
	ErrClusterOffline = errors.New("the cluster is disabled in the registry")
)

type FailureCode string

const (
	FailUnreachable  FailureCode = "k8s.unreachable"
	FailTLS          FailureCode = "k8s.tls_error"
	FailUnauthorized FailureCode = "k8s.unauthorized"
	FailForbidden    FailureCode = "k8s.forbidden"
	FailTimeout      FailureCode = "k8s.timeout"

	FailAgentSilent        FailureCode = "k8s.agent_silent"
	FailAgentNeverReported FailureCode = "k8s.agent_never_reported"
)

func ReportedFailure(code string) bool {
	switch FailureCode(code) {
	case FailUnreachable, FailTLS, FailUnauthorized, FailForbidden, FailTimeout:
		return true
	}
	return false
}

type Failure struct {
	Code    FailureCode
	Message string
}

func (f *Failure) Error() string { return fmt.Sprintf("%s: %s", f.Code, f.Message) }
