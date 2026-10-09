package apperr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"svc-registry/internal/access"
	"svc-registry/internal/catalog"
	"svc-registry/internal/deploy"
	"svc-registry/internal/forge"
	"svc-registry/internal/ingest"
	"svc-registry/internal/knowledge"
	"svc-registry/internal/links"
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

func From(err error) *Error {
	if err == nil {
		return nil
	}
	var app *Error
	if errors.As(err, &app) {
		return app
	}
	var coded interface {
		Code() string
		Message() string
	}
	var accessInvalid access.Invalid
	var catalogInvalid catalog.Invalid
	var forgeInvalid forge.Invalid
	var linksInvalid links.Invalid
	var linksTemplate *links.TemplateError
	var linksConflict links.Conflict
	var deployInvalid deploy.Invalid
	var deployConflict deploy.Conflict
	var knowledgeInvalid knowledge.Invalid
	var knowledgeConflict knowledge.Conflict
	var forgeConflict forge.Conflict
	var accessConflict access.Conflict
	var catalogConflict catalog.Conflict
	var ingestConflict ingest.Conflict
	var rejection *ingest.Rejection
	switch {
	case errors.As(err, &accessInvalid):
		coded = accessInvalid
	case errors.As(err, &catalogInvalid):
		coded = catalogInvalid
	case errors.As(err, &forgeInvalid):
		coded = forgeInvalid
	case errors.As(err, &linksTemplate):
		coded = linksTemplate
	case errors.As(err, &linksInvalid):
		coded = linksInvalid
	case errors.As(err, &deployInvalid):
		coded = deployInvalid
	case errors.As(err, &knowledgeInvalid):
		coded = knowledgeInvalid
	}
	if coded != nil {
		return &Error{Kind: Validation, Code: coded.Code(), Message: coded.Message()}
	}
	switch {
	case errors.As(err, &accessConflict):
		coded = accessConflict
	case errors.As(err, &catalogConflict):
		coded = catalogConflict
	case errors.As(err, &ingestConflict):
		coded = ingestConflict
	case errors.As(err, &forgeConflict):
		coded = forgeConflict
	case errors.As(err, &linksConflict):
		coded = linksConflict
	case errors.As(err, &deployConflict):
		coded = deployConflict
	case errors.As(err, &knowledgeConflict):
		coded = knowledgeConflict
	}
	if coded != nil {
		return &Error{Kind: Conflict, Code: coded.Code(), Message: coded.Message()}
	}
	if errors.As(err, &rejection) {
		e := &Error{Kind: Unprocessable, Code: rejection.Code(), Message: rejection.Message()}
		for _, f := range rejection.Fields {
			e.Fields = append(e.Fields, Field{Path: f.Path, Code: string(f.Code)})
		}
		return e
	}
	if code := forge.FailureCode(err); code != "" {
		return fromForge(err, code)
	}
	var accessInternal *access.InternalError
	var catalogInternal *catalog.InternalError
	var ingestInternal *ingest.InternalError
	switch {
	case errors.Is(err, knowledge.ErrSearchUnavailable):
		return &Error{Kind: Unavailable, Code: "search.unavailable", What: "search engine"}
	case errors.Is(err, access.ErrNotFound), errors.Is(err, catalog.ErrNotFound), errors.Is(err, forge.ErrNotFound),
		errors.Is(err, links.ErrNotFound), errors.Is(err, deploy.ErrNotFound), errors.Is(err, knowledge.ErrNotFound):
		return New(NotFound)
	case errors.Is(err, access.ErrUnavailable), errors.Is(err, catalog.ErrUnavailable),
		errors.Is(err, ingest.ErrUnavailable), errors.Is(err, forge.ErrUnavailable), errors.Is(err, links.ErrUnavailable), errors.Is(err, deploy.ErrUnavailable), errors.Is(err, knowledge.ErrUnavailable),
		IsUnavailable(err):
		return UnavailableDB()
	case errors.As(err, &accessInternal):
		return &Error{Kind: Internal, Detail: accessInternal.Detail}
	case errors.As(err, &catalogInternal):
		return &Error{Kind: Internal, Detail: catalogInternal.Detail}
	case errors.As(err, &ingestInternal):
		return &Error{Kind: Internal, Detail: ingestInternal.Detail}
	}
	var forgeInternal *forge.InternalError
	if errors.As(err, &forgeInternal) {
		return &Error{Kind: Internal, Detail: forgeInternal.Detail}
	}
	var linksInternal *links.InternalError
	if errors.As(err, &linksInternal) {
		return &Error{Kind: Internal, Detail: linksInternal.Detail}
	}
	var deployInternal *deploy.InternalError
	if errors.As(err, &deployInternal) {
		return &Error{Kind: Internal, Detail: deployInternal.Detail}
	}
	var knowledgeInternal *knowledge.InternalError
	if errors.As(err, &knowledgeInternal) {
		return &Error{Kind: Internal, Detail: knowledgeInternal.Detail}
	}
	return &Error{Kind: Internal, Detail: err.Error()}
}

func fromForge(err error, code string) *Error {
	switch code {
	case "forge.rate_limited":
		e := &Error{Kind: UpstreamRateLimited, Code: code, Message: err.Error()}
		var limited *forge.RateLimited
		if errors.As(err, &limited) && !limited.Reset.IsZero() {
			if secs := time.Until(limited.Reset).Seconds(); secs > 0 {
				e.RetryAfterSecs = uint64(secs) + 1
			}
		}
		return e
	case "forge.upstream_error":
		return &Error{Kind: BadGateway, Code: code, Message: err.Error()}
	case "forge.interrupted":
		return UnavailableDB()
	}
	return &Error{Kind: Unprocessable, Code: code, Message: err.Error()}
}

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
