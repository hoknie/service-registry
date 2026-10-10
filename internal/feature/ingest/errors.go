package ingest

import (
	"svc-registry/internal/platform/apperr"
)

type FieldCode string

const (
	FieldRequired     FieldCode = "required"
	FieldInvalidType  FieldCode = "invalid_type"
	FieldInvalidValue FieldCode = "invalid_value"
	FieldInFuture     FieldCode = "in_future"
)

type FieldError struct {
	Path string
	Code FieldCode
}

type RejectionKind int

const (
	RejectUnknownType RejectionKind = iota + 1
	RejectUnsupportedVersion
	RejectInvalid
)

type Rejection struct {
	Kind   RejectionKind
	Fields []FieldError
}

func (r *Rejection) Code() string {
	switch r.Kind {
	case RejectUnknownType:
		return "ingest.unknown_event_type"
	case RejectUnsupportedVersion:
		return "ingest.unsupported_version"
	}
	return "ingest.invalid_event"
}

func (r *Rejection) Message() string {
	switch r.Kind {
	case RejectUnknownType:
		return "unknown event type"
	case RejectUnsupportedVersion:
		return "this version of the event type is not supported"
	}
	return "the event has invalid fields"
}

func (r *Rejection) Error() string { return r.Message() }

func Invalid(fields ...FieldError) *Rejection { return &Rejection{Kind: RejectInvalid, Fields: fields} }

type Conflict int

const (
	ConflictIdempotencyKeyReused Conflict = iota + 1
)

func (c Conflict) Code() string { return "conflict.idempotency_key_reused" }

func (c Conflict) Message() string {
	return "this idempotency_key was already used for a different event of the project"
}

func (c Conflict) Error() string { return c.Message() }

var ErrUnavailable = apperr.Sentinel(apperr.Unavailable, "database is unavailable")

type InternalError struct{ Detail string }

func (e *InternalError) Error() string { return "internal: " + e.Detail }

func (c Conflict) AppError() *apperr.Error { return apperr.Conflicting(c.Code(), c.Message()) }

func (e *InternalError) AppError() *apperr.Error { return apperr.InternalDetail(e.Detail) }

func (r *Rejection) AppError() *apperr.Error {
	e := &apperr.Error{Kind: apperr.Unprocessable, Code: r.Code(), Message: r.Message()}
	for _, f := range r.Fields {
		e.Fields = append(e.Fields, apperr.Field{Path: f.Path, Code: string(f.Code)})
	}
	return e
}
