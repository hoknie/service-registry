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
	}
	return e.Message
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
	var app *Error
	if errors.As(err, &app) {
		return app
	}
	var coded Coded
	if errors.As(err, &coded) {
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

func Conflicting(code, message string) *Error {
	return &Error{Kind: Conflict, Code: code, Message: message}
}

func InternalDetail(detail string) *Error { return &Error{Kind: Internal, Detail: detail} }

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
	var connect *pgconn.ConnectError
	var netErr net.Error
	return errors.As(err, &connect) || errors.As(err, &netErr) ||
		errors.Is(err, context.DeadlineExceeded) || pgconn.Timeout(err)
}

func Wrap(err error) error {
	if e := From(err); e != nil {
		return e
	}
	return nil
}
