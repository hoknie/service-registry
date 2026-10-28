package apperr

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/jackc/pgx/v5/pgconn"
)

type Kind int

const (
	NotFound Kind = iota + 1
	Unavailable
	Internal
	Unauthenticated
	InvalidCredentials
	Forbidden
	AccountDisabled
	WrongPassword
	RateLimited
	Validation
	Conflict
	InvalidProjectKey
	IngestRateLimited
	Unprocessable
	InvalidToken
	InsufficientScope
	SessionRequired
	UpstreamRateLimited
	BadGateway
	InvalidWebhookSignature
	InvalidAgentToken
	PublicRateLimited
	TooLarge
	Refused
)

type Field struct {
	Path string
	Code string
}

type Error struct {
	Kind           Kind
	Code           string
	Message        string
	What           string
	Detail         string
	RetryAfterSecs uint64
	Fields         []Field
}

func (e *Error) Error() string {
	switch e.Kind {
	case NotFound:
		return "not found"
	case Unavailable:
		return e.What + " is unavailable"
	case Internal:
		return "internal: " + e.Detail
	case Unauthenticated:
		return "authentication required"
	case InvalidCredentials:
		return "invalid email or password"
	case Forbidden:
		return "not allowed"
	case AccountDisabled:
		return "account is disabled"
	case WrongPassword:
		return "current password is wrong"
	case RateLimited:
		return fmt.Sprintf("too many failed sign-in attempts; retry in %d s", e.RetryAfterSecs)
	case InvalidProjectKey:
		return "a valid project key for this project is required"
	case IngestRateLimited:
		return fmt.Sprintf("too many events with this key; retry in %d s", e.RetryAfterSecs)
	case InvalidToken:
		return "a valid personal access token is required"
	case InsufficientScope:
		return "the token's scopes do not allow this request"
	case SessionRequired:
		return "this request needs a session, not a token"
	case UpstreamRateLimited:
		return "the forge's rate limit is exceeded"
	case InvalidWebhookSignature:
		return "the webhook delivery is not signed by this connection"
	case InvalidAgentToken:
		return "a valid agent token for this cluster is required"
	case PublicRateLimited:
		return fmt.Sprintf("too many requests to the public documentation; retry in %d s", e.RetryAfterSecs)
	}
	return e.Message
}

var (
	ErrNotFound    = &Error{Kind: NotFound}
	ErrUnavailable = &Error{Kind: Unavailable, What: "database"}
)

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	if t == ErrNotFound || t == ErrUnavailable {
		return e.Kind == t.Kind
	}
	return e == t
}

func New(kind Kind) *Error { return &Error{Kind: kind} }

func Internalf(format string, args ...any) *Error {
	return &Error{Kind: Internal, Detail: fmt.Sprintf(format, args...)}
}

func UnavailableDB() *Error { return &Error{Kind: Unavailable, What: "database"} }

func RateLimitedFor(secs uint64) *Error { return &Error{Kind: RateLimited, RetryAfterSecs: secs} }

type Coded interface {
	error
	AppError() *Error
}

func From(err error) *Error {
	if err == nil {
		return nil
	}
	if app, ok := errors.AsType[*Error](err); ok {
		return app
	}
	if coded, ok := errors.AsType[Coded](err); ok {
		return coded.AppError()
	}
	if IsUnavailable(err) {
		return UnavailableDB()
	}
	return &Error{Kind: Internal, Detail: err.Error()}
}

func Invalid(code, message string) *Error {
	return &Error{Kind: Validation, Code: code, Message: message}
}

func TooLargeError(code, message string) *Error {
	return &Error{Kind: TooLarge, Code: code, Message: message}
}

func Refusing(code, message string) *Error {
	return &Error{Kind: Refused, Code: code, Message: message}
}

func Conflicting(code, message string) *Error {
	return &Error{Kind: Conflict, Code: code, Message: message}
}

func InternalDetail(detail string) *Error { return &Error{Kind: Internal, Detail: detail} }

func UnknownValue(what, value string) *Error { return InternalDetail("unknown " + what + " " + value) }

type sentinel struct {
	kind    Kind
	message string
}

func (s *sentinel) Error() string { return s.message }

func (s *sentinel) AppError() *Error {
	if s.kind == Unavailable {
		return UnavailableDB()
	}
	return New(s.kind)
}

func Sentinel(kind Kind, message string) error { return &sentinel{kind: kind, message: message} }

func IsUnavailable(err error) bool {
	if _, ok := errors.AsType[*pgconn.ConnectError](err); ok {
		return true
	}
	if _, ok := errors.AsType[net.Error](err); ok {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded) || pgconn.Timeout(err)
}

func Wrap(err error) error {
	if e := From(err); e != nil {
		return e
	}
	return nil
}
