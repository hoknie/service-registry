package ingest

import "encoding/json"

const (
	BodyLimit         = 65_536
	IdempotencyKeyMax = 200
)

type WireEnvelope struct {
	Type           Field[string]            `json:"type"`
	Version        Field[int64]             `json:"version"`
	OccurredAt     Field[string]            `json:"occurred_at"`
	IdempotencyKey Field[string]            `json:"idempotency_key"`
	Payload        Field[ServiceDeployedV1] `json:"payload"`
}

var (
	occurredAtRule = rule{tags: "datetime=2006-01-02T15:04:05Z07:00"}
	keyRule        = rule{tags: "max=200,visibleascii"}
)

func ParseEnvelope(w WireEnvelope) (Envelope, error) {
	var errs fields
	switch {
	case w.Type.State == WrongType:
		return Envelope{}, Invalid(FieldError{"type", FieldInvalidType})
	case missing(w.Type):
		return Envelope{}, Invalid(FieldError{"type", FieldRequired})
	}
	typ, ok := ParseEventType(w.Type.Value)
	if !ok {
		return Envelope{}, &Rejection{Kind: RejectUnknownType}
	}
	switch w.Version.State {
	case Absent, Null:
		return Envelope{}, Invalid(FieldError{"version", FieldRequired})
	case WrongType:
		return Envelope{}, Invalid(FieldError{"version", FieldInvalidType})
	}
	if !typ.Supports(w.Version.Value) {
		return Envelope{}, &Rejection{Kind: RejectUnsupportedVersion}
	}

	occurredAt := errs.required("occurred_at", w.OccurredAt, occurredAtRule)
	key := errs.required("idempotency_key", w.IdempotencyKey, keyRule)
	var payload ServiceDeployed
	switch w.Payload.State {
	case Absent, Null:
		errs.add("payload", FieldRequired)
	case WrongType:
		errs.add("payload", FieldInvalidType)
	default:
		payload = w.Payload.Value.parse(&errs)
	}
	if len(errs) > 0 {
		return Envelope{}, Invalid(errs...)
	}
	return Envelope{
		Type: typ, Version: int32(w.Version.Value), OccurredAt: occurredAt, IdempotencyKey: key,
		ServiceDeployed: &payload, Payload: json.RawMessage(w.Payload.Raw),
	}, nil
}
