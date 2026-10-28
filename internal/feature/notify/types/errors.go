package types

import "svc-registry/internal/platform/apperr"

var (
	InvalidSubscription    = apperr.Invalid("validation.invalid_subscription", "a subscription needs channel webhook or slack, secret_id unless disabled, 1 to 7 known event_types without repeats, environments null or at most 20 environment names and locale en, es, ru or zh")
	TooManySubscriptions   = apperr.Invalid("validation.too_many_subscriptions", "a node has at most 20 notification subscriptions")
	InvalidDeliveryFilter  = apperr.Invalid("validation.invalid_filter", "subscription must be a subscription key and status pending, delivered or failed")
	RecoveryWithoutFailure = apperr.Invalid("validation.recovery_without_failure", "a recovery event type needs its failure type in event_types: deployment.drift_resolved with deployment.drift, link.check_recovered with link.check_failed, repo_sync.recovered with repo_sync.failed")
	InvalidNotificationURL = apperr.Invalid("validation.invalid_notification_url", "the secret must hold an absolute http(s) URL of at most 2048 characters; for slack an https://hooks.slack.com/ address")
)

func RateLimited(secs uint64) *apperr.Error {
	return &apperr.Error{Kind: apperr.UpstreamRateLimited, Code: "notify.rate_limited",
		Message: "a test notification of this subscription was sent less than 10 seconds ago", RetryAfterSecs: secs}
}
