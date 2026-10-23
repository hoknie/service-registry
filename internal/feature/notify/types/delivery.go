package types

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusDelivered Status = "delivered"
	StatusFailed    Status = "failed"
)

const (
	CodeHTTPStatus           = "notify.http_status"
	CodeTimeout              = "notify.timeout"
	CodeTLSError             = "notify.tls_error"
	CodeDNSError             = "notify.dns_error"
	CodeUnreachable          = "notify.unreachable"
	CodeBlocked              = "notify.blocked"
	CodeRedirect             = "notify.redirect"
	CodeURLUnavailable       = "notify.url_unavailable"
	CodeSubscriptionDisabled = "notify.subscription_disabled"
)

var pauses = []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute, time.Hour,
	3 * time.Hour, 6 * time.Hour, 12 * time.Hour}

const maxRetryAfter = time.Hour

type Delivery struct {
	ID               uuid.UUID
	EventID          uuid.UUID
	EventType        EventType
	EventNodeID      uuid.UUID
	EventNodePath    *string
	SubscriptionKey  string
	Channel          Channel
	Status           Status
	Attempts         int32
	NextAttemptAt    *string
	LastHTTPStatus   *int32
	LastErrorCode    *string
	LastErrorMessage *string
	CreatedAt        string
	DeliveredAt      *string
}

type DeliveryFilter struct {
	Subscription *string
	Status       *string
}

type Request struct {
	URL     string
	Body    []byte
	Headers map[string]string
}

type Response struct {
	HTTPStatus   *int
	RetryAfter   time.Duration
	ErrorCode    *string
	ErrorMessage *string
	Retryable    bool
	DurationMS   int
}

func (r Response) Delivered() bool { return r.ErrorCode == nil && r.HTTPStatus != nil }

type Outcome struct {
	Status        Status
	NextAttemptIn time.Duration
	HTTPStatus    *int
	ErrorCode     *string
	ErrorMessage  *string
}

func Decide(r Response, attempts, maxAttempts int) Outcome {
	o := Outcome{HTTPStatus: r.HTTPStatus, ErrorCode: r.ErrorCode, ErrorMessage: r.ErrorMessage}
	switch {
	case r.Delivered():
		o.Status = StatusDelivered
	case !r.Retryable || attempts >= maxAttempts:
		o.Status = StatusFailed
	default:
		o.Status = StatusPending
		o.NextAttemptIn = Pause(attempts, r.RetryAfter)
	}
	return o
}

func Pause(attempts int, retryAfter time.Duration) time.Duration {
	p := pauses[min(max(attempts, 1), len(pauses))-1]
	return max(p, min(retryAfter, maxRetryAfter))
}

func Retryable(status int) bool {
	return status == 408 || status == 429 || status >= 500
}

func ValidStatus(s string) bool {
	switch Status(s) {
	case StatusPending, StatusDelivered, StatusFailed:
		return true
	}
	return false
}

type Job struct {
	ID             uuid.UUID
	SubscriptionID uuid.UUID
	Status         Status
	Attempts       int32
	Payload        []byte
}

type TestResult struct {
	Status     Status
	HTTPStatus *int
	ErrorCode  *string
	DurationMS int
}
